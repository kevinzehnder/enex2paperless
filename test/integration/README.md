# Integration Tests

This directory contains integration tests that run against an actual Paperless-NGX instance.

## Overview

These tests validate the complete upload workflow by:
- Using real `.enex` files from the `assets/` directory
- Uploading documents to a running Paperless instance
- Verifying documents and tags are created correctly
- Testing file type filtering, ZIP processing, and concurrent uploads

## Requirements

### 1. Running Paperless-NGX Instance

You need a Paperless-NGX instance running and accessible. The easiest way is using Docker:

```bash
docker run -d \
  --name paperless-test \
  -p 8000:8000 \
  -e PAPERLESS_SECRET_KEY=test-secret-key \
  -e PAPERLESS_ADMIN_USER=admin \
  -e PAPERLESS_ADMIN_PASSWORD=admin \
  ghcr.io/paperless-ngx/paperless-ngx:latest
```

Wait for Paperless to start up (usually 30-60 seconds), then create an API token:

```bash
# Access the container
docker exec -it paperless-test bash

# Create a token for the admin user
python3 manage.py shell -c "from django.contrib.auth.models import User; from rest_framework.authtoken.models import Token; user = User.objects.get(username='admin'); token, created = Token.objects.get_or_create(user=user); print(f'Token: {token.key}')"
```

### 2. Environment Variables

Set the following environment variables to configure the tests:

```bash
# Required: Paperless API URL
export E2P_PAPERLESSAPI="http://localhost:8000"

# Authentication: Use either token OR username+password
export E2P_TOKEN="your-api-token-here"

# OR use basic auth:
export E2P_USERNAME="admin"
export E2P_PASSWORD="admin"
```

## Running the Tests

Integration tests use build tags to prevent them from running during normal `go test`:

```bash
# Run all integration tests
go test -tags=integration ./test/integration/...

# Run with verbose output
go test -v -tags=integration ./test/integration/...

# Run a specific test
go test -v -tags=integration ./test/integration/... -run TestBasicDocumentUpload

# Run with race detector
go test -v -race -tags=integration ./test/integration/...
```

## Test Cases

### TestBasicDocumentUpload
- Uploads a single PDF document from `test.enex`
- Verifies document title and tags are correct
- Cleans up after completion

### TestDocumentWithMultipleTags
- Tests the `AdditionalTags` configuration feature
- Verifies all tags (from ENEX + additional) are applied
- Validates tag creation and association

### TestFileTypeFiltering
- Tests file type filtering with `filetypes.enex`
- Verifies only allowed file types are uploaded
- Uses `FileTypes` configuration

### TestZipFileProcessing
- Tests ZIP file extraction and upload
- Verifies files are extracted from ZIP archives
- Validates individual file uploads

### TestConcurrentUploads
- Tests concurrent upload workers
- Verifies thread-safety of upload process
- Validates correct handling of parallel uploads

### TestUploadMetrics
- Validates upload metrics (NumNotes, Uploads)
- Verifies counters are updated correctly
- Checks processing statistics

## Architecture

### Dependency Injection
Tests use the newly implemented dependency injection pattern:

```go
cfg := GetTestConfig(t)
enexFile := enex.NewEnexFile(enexPath, cfg)
```

This makes tests:
- ✅ **Isolated** - Each test has its own config
- ✅ **Flexible** - Easy to override configuration
- ✅ **Maintainable** - No global state

### Paperless Client
A minimal client (`paperless_client.go`) provides verification methods:

```go
client := GetPaperlessClient(t, cfg)
doc, err := client.GetDocumentByTitle("My Document")
AssertDocumentHasTag(t, client, doc, "MyTag")
```

### Test Helpers
Helper functions (`helpers.go`) provide common functionality:

- `GetTestConfig()` - Creates test configuration
- `SkipIfPaperlessUnavailable()` - Skips if Paperless is down
- `CleanupTestDocuments()` - Removes test documents
- `CleanupTestTags()` - Removes test tags
- `AssertDocumentExists()` - Verifies document presence
- `AssertDocumentHasTag()` - Verifies tag association

## Cleanup

Tests automatically clean up after themselves using `defer`:

```go
defer CleanupTestDocuments(t, client, "Test PDF Note")
defer CleanupTestTags(t, client, []string{"SampleTag"})
```

However, if a test crashes, manual cleanup may be needed:

```bash
# List all documents
curl -H "Authorization: Token YOUR_TOKEN" http://localhost:8000/api/documents/

# Delete a document
curl -X DELETE -H "Authorization: Token YOUR_TOKEN" http://localhost:8000/api/documents/123/

# List all tags
curl -H "Authorization: Token YOUR_TOKEN" http://localhost:8000/api/tags/

# Delete a tag
curl -X DELETE -H "Authorization: Token YOUR_TOKEN" http://localhost:8000/api/tags/456/
```

## CI/CD Integration

To run integration tests in CI:

```yaml
# Example GitHub Actions workflow
- name: Start Paperless
  run: |
    docker run -d \
      --name paperless \
      -p 8000:8000 \
      -e PAPERLESS_SECRET_KEY=ci-secret \
      -e PAPERLESS_ADMIN_USER=admin \
      -e PAPERLESS_ADMIN_PASSWORD=admin \
      ghcr.io/paperless-ngx/paperless-ngx:latest
    
- name: Wait for Paperless
  run: |
    timeout 60 bash -c 'until curl -f http://localhost:8000/api/ 2>/dev/null; do sleep 2; done'

- name: Create API Token
  run: |
    TOKEN=$(docker exec paperless python3 manage.py shell -c "from django.contrib.auth.models import User; from rest_framework.authtoken.models import Token; user = User.objects.get(username='admin'); token, _ = Token.objects.get_or_create(user=user); print(token.key)")
    echo "E2P_TOKEN=$TOKEN" >> $GITHUB_ENV

- name: Run Integration Tests
  run: go test -v -tags=integration ./test/integration/...
  env:
    E2P_PAPERLESSAPI: http://localhost:8000
```

## Troubleshooting

### "Paperless instance not available"
- Check if Paperless is running: `curl http://localhost:8000/api/`
- Verify the `E2P_PAPERLESSAPI` URL is correct
- Check Docker logs: `docker logs paperless-test`

### "Integration tests require authentication"
- Ensure either `E2P_TOKEN` or `E2P_USERNAME`+`E2P_PASSWORD` is set
- Verify token is valid: `curl -H "Authorization: Token YOUR_TOKEN" http://localhost:8000/api/documents/`

### "Document not found after upload"
- Check Paperless logs for errors
- Increase timeout in `WaitForDocument()` calls
- Verify file permissions on `.enex` files

### Tests hang indefinitely
- Check for goroutine leaks
- Verify channels are being closed properly
- Use `-timeout` flag: `go test -timeout 2m -tags=integration ./test/integration/...`

## Development

When adding new integration tests:

1. **Add build tag**: Start file with `//go:build integration`
2. **Use helpers**: Leverage `GetTestConfig()`, `GetPaperlessClient()`, etc.
3. **Clean up**: Always use `defer` to clean up resources
4. **Use assertions**: Use `AssertDocumentExists()` and `AssertDocumentHasTag()`
5. **Add timeouts**: Use `WaitForDocument()` with reasonable timeouts
6. **Document**: Update this README with new test cases

## Benefits

✅ **Real-world validation** - Tests against actual Paperless instance  
✅ **Full workflow coverage** - End-to-end upload and verification  
✅ **Dependency injection** - Clean, testable architecture  
✅ **Isolated tests** - No global state, independent configs  
✅ **Automatic cleanup** - Tests clean up after themselves  
✅ **CI/CD ready** - Can run in automated pipelines  
