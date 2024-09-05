# enex2paperless

## Description

CLI tool to help migrate attachements from Evernote notes to Paperless-NGX. It parses ENEX files and uses the Paperless API to upload the contents.

I've been using Evernote as a filing cabinet with mostly notes containing a single PDF file and various tags. This tool is specifically built to ingest these types of notes from Evernote to Paperless.

**What it does:**

- Go through an ENEX file, looking for notes containing allowed file types.
- Extract those files and upload them to Paperless

It will recreate the same tags and note title as they were in Evernote.

**What it doesn't do:**

It will not convert ALL your existing notes. Notes without allowed attachements will be ignored.

## How To Use

```shell
Usage:
  enex2paperless [file path] [flags]

Flags:
  -c, --concurrent int   Number of concurrent consumers (default 1)
  -h, --help             help for enex2paperless
  -v, --verbose          Enable verbose logging
```

### Windows

- Export your Notes from Evernote to an ENEX file, e.g. `MyEnexFile.enex`

- Download `enex2paperless.zip` from [Releases](https://github.com/kevinzehnder/enex2paperless/releases/latest).
- Extract files to the same location as your ENEX file.
- Edit `config.yaml` and add your personal information. This depends on your installation of Paperless:

```yaml
PaperlessAPI: http://paperboy.lan:8000
Username: user
Password: pass
FileTypes:
  - pdf
  - txt
  - jpeg
  - png
  - webp
  - gif
  - tiff
```

- Open `cmd.exe`
- Navigate to the folder where you extracted the files, e.g.: `cd C:\Users\JohnDoe\Desktop`
- Run `enex2paperless`:

```shell
enex2paperless MyEnexFile.enex
```

## Additional Configuration Options

### Allowed FileTypes

You can select which file types should be processed. The MIME type of the Evernote attachements will be compared with the configured file types. If there's no match, the attachement will be ignored. This avoids trying to upload unwanted or unsupported filetypes.

If you want to upload any attachements, regardless, then you can include `any` in the list of filetypes, like this:

```yaml
FileTypes:
  - any
```

### Multiple Concurrent Uploads

The tool is capable of handling multiple uploads concurrently. By default it will process the attachements one by one. You can use the `-c` flag to configure multiple workers, like this:

```shell
enex2paperless.exe MyEnexFile.enex -c 3
```

> **Attention:** Depending on your Paperless installation, it might not be able to handle multiple requests at the same time efficiently. In that case, using multiple concurrent uploads would only slow down the process instead of speeding it up.

### Verbose Logging

If you're running into problems, you can enable a more verbose log output by using the `-v` flag. This should help troubleshoot the problems.
