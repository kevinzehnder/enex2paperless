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
  -c, --concurrent int        Number of concurrent consumers (default 1)
  -h, --help                  help for enex2paperless
  -n, --nocolor               Disable colored output
  -o, --outputfolder string   Output attachements to this folder, NOT paperless.
  -t, --tags strings          Additional tags to add to all documents.
  -T, --use-filename-tag      Add the ENEX filename as tag to all documents.
  -v, --verbose               Enable verbose logging
```

### Example using Windows

- Export your Notes from Evernote to an ENEX file, e.g. `MyEnexFile.enex`

- Download `enex2paperless.zip` from [Releases](https://github.com/kevinzehnder/enex2paperless/releases/latest).
- Extract files to the same location as your ENEX file.
- Edit `config.yaml` and add your personal information. This depends on your installation of Paperless:

```yaml
PaperlessAPI: http://paperboy.lan:8000
Username: user
Password: pass
Token: 
FileTypes:
  - pdf
  - txt
  - jpeg
  - png
  - webp
  - gif
  - tiff
  - zip
```

To authenticate against Paperless, you can either use a token or a username/password combination. Don't configure both variations at the same time. 

- Open `cmd.exe`
- Navigate to the folder where you extracted the files, e.g.: `cd C:\Users\JohnDoe\Desktop`
- Run `enex2paperless`:

```shell
enex2paperless MyEnexFile.enex
```

## Additional Configuration Options

### 1. Allowed FileTypes

You can select which file types should be processed. The MIME type of the Evernote attachements will be compared with the configured file types. If there's no match, the attachement will be ignored. This avoids trying to upload unwanted or unsupported filetypes.

If you want to upload any attachements, regardless, then you can include `any` in the list of filetypes, like this:

```yaml
FileTypes:
  - any
```

If `zip` files are allowed, they will get extracted and the contents will be processed individually. 

### 2. Multiple Concurrent Uploads

The tool is capable of handling multiple uploads concurrently. By default it will process the attachements one by one. You can use the `-c` flag to configure multiple workers, like this:

```shell
enex2paperless.exe MyEnexFile.enex -c 3
```

> **Attention:** Depending on your Paperless installation, it might not be able to handle multiple requests at the same time efficiently. In that case, using multiple concurrent uploads would only slow down the process instead of speeding it up.

### 3. Output To Folder

Optionally it is possible to output all attachements to a specific folder, as opposed to uploading them to Paperless. If you want to use Enex2Paperless in that mode, then you have to provide a foldername:

```shell
enex2paperless.exe MyEnexFile.enex -o myfoldername
```

This disables uploads to Paperless and only outputs files to your provided folder.
If `zip` files are allowed, this will create a subfolder for each zip file, named after the zip file (without the .zip extension), and extract its contents there.

### 4. Additional Tags / Filename As Tag

You can add additional tags to all files being processed using the `-t` flag and a comma separated list of strings:

```shell
enex2paperless.exe MyEnexFile.enex -t migration2024,taxes,important
```

If you set the `-T` flag, the ENEX filename (without extension) will be used as an additional tag:

```shell
enex2paperless.exe MyEnexFile.enex -T
```

This will add "MyEnexFile" as a tag to all processed files. 

If you use neither the `-t` or `-T` flags, no additional tags will be added, and only the original Evernote tags will be preserved.


### 5. Verbose Logging

If you're running into problems, you can enable a more verbose log output by using the `-v` flag. This should help troubleshoot the problems.

### 6. NoColor

If your console doesn't support colored output using ANSI escape codes, the output will look messed up, similar to this:

```shell
←[38;2;224;175;104m[08:46:55.211]←[0m [←[38;2;158;206;105m INFO←[0m] ←[38;2;192;202;245mprocessing file←[0m ←[38;2;187;154;247m{"file":"test.pdf"}←[0m
←[38;2;224;175;104m[08:46:56.734]←[0m [←[38;2;158;206;105m INFO←[0m] ←[38;2;192;202;245mENEX processing done←[0m ←[38;2;187;154;247m{"numberOfNotes":1,"totalFiles":1}←[0m
←[38;2;224;175;104m[08:46:56.734]←[0m [←[38;2;158;206;105m INFO←[0m] ←[38;2;192;202;245mall notes processed successfully←[0m
```

If that's the case, enable the `-n` flag to disable colored output.
