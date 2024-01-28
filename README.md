# enex2paperless

CLI tool to help migrate PDFs from Evernote to Paperless-NGX. It parses ENEX files and uses the Paperless API to upload the contents.

Status: Alpha

## Installation and Configuration

Copy enex2paperless binary and config.yaml to a folder.

Modify config.yaml with your information:

```yaml
PaperlessAPI: http://paperboy.lan:8000
Username: user
Password: pass
```

## Usage

```
enex2paperless [file] [flags]
```
