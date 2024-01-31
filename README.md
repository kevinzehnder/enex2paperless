# enex2paperless

## Description

CLI tool to help migrate PDFs from Evernote to Paperless-NGX. It parses ENEX files and uses the Paperless API to upload the contents.

Status: Alpha

I've been using Evernote as a filing cabinet with mostly notes containing a single PDF file and various tags. This tool is specifically built to ingest these types of notes from Evernote to Paperless.

**What it does:**

- Go through an ENEX file, looking for notes containing pdf files.
- Extract those PDF files and upload them to Paperless

It will recreate the same tags and note title as they were in Evernote.

**What it doesn't do:**

It will not convert ALL your existing notes. Notes without PDF files will be ignored.

## How To Use

### Windows

- Export your Notes from Evernote to an ENEX file, e.g. `MyEnexFile.enex`

- Download `enex2paperless.zip` from [Releases](https://github.com/kevinzehnder/enex2paperless/releases/latest).
- Extract files to the same location as your ENEX file.
- Edit config.yaml and add your personal information. This depends on your installation of Paperless:

```yaml
PaperlessAPI: http://paperboy.lan:8000
Username: user
Password: pass
```

- Open `cmd.exe`
- Navigate to the folder where you extracted the files, e.g.: `cd C:\Users\JohnDoe\Desktop`
- Run `enex2paperless`:

```shell
enex2paperless MyEnexFile.enex
```
