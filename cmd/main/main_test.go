package main

import (
	"enex2paperless/pkg/enex"
	"sync"
	"testing"

	"github.com/spf13/afero"
)

type testCase struct {
	Name          string
	MockEnexData  string
	ExpectedError error
	ExpectedNotes int
}

var testCases = []testCase{
	{
		Name: "Basic Note Parsing",
		MockEnexData: `
		<?xml version="1.0" encoding="UTF-8"?>
		<!DOCTYPE en-export SYSTEM "http://xml.evernote.com/pub/evernote-export3.dtd">
		<en-export export-date="2023-01-01T12:34:56Z" application="MockApp" version="1.0">
			<note>
				<title>Test Note</title>
				<content><![CDATA[<?xml version="1.0" encoding="UTF-8"?>
					<!DOCTYPE en-note SYSTEM "http://xml.evernote.com/pub/enml2.dtd">
					<en-note>
						<div>This is a test note.</div>
					</en-note>
				]]></content>
				<created>2023-01-01T01:01:01Z</created>
				<updated>2023-01-02T02:02:02Z</updated>
				<tag>SampleTag</tag>
				<note-attributes>
					<latitude>47.6062</latitude>
					<longitude>-122.3321</longitude>
					<altitude>21.0</altitude>
					<author>Test Author</author>
				</note-attributes>
				<resource>
					<data>Base64EncodedDataHere</data>
					<mime>application/pdf</mime>
					<resource-attributes>
						<source-url>file://test.pdf</source-url>
						<file-name>test.pdf</file-name>
					</resource-attributes>
				</resource>
			</note>
		</en-export>
		`,
		ExpectedError: nil,
		ExpectedNotes: 1,
	},
	{
		Name: "Multiple Notes",
		MockEnexData: `
		<!-- ENEX with Multiple Notes -->
		<!-- Note 1 -->
		<note>
			<title>Note 1 Title</title>
			<!-- ... -->
		</note>
		<!-- Note 2 -->
		<note>
			<title>Note 2 Title</title>
			<!-- ... -->
		</note>
	`,
		ExpectedError: nil,
		ExpectedNotes: 2,
	},
	{
		Name: "Missing Content",
		MockEnexData: `
		<!-- ENEX with Missing Content -->
		<note>
			<title>Title Here</title>
			<title>Title Here</title>
			<!-- No content here -->
		</note>
	`,
		ExpectedError: nil,
		ExpectedNotes: 1,
	},
	{
		Name: "Malformed XML",
		MockEnexData: `
		&nbsp;
		`, // Missing closing tags for en-export
		ExpectedError: nil,
		ExpectedNotes: 0,
	},
	{
		Name:          "No Notes",
		MockEnexData:  `<!-- ENEX with No Notes -->`,
		ExpectedError: nil,
		ExpectedNotes: 0,
	},
}

func TestReadFromFile(t *testing.T) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			// prepare RAM filesystem
			mockFs := afero.NewMemMapFs()
			afero.WriteFile(mockFs, "test.enex", []byte(tc.MockEnexData), 0644)

			// Create an EnexFile with channels
			enexFile := enex.NewEnexFile("test.enex")
			enexFile.Fs = mockFs

			// Use a wait group to synchronize test
			var wg sync.WaitGroup
			wg.Add(1)

			// Create results slice
			var results []enex.Note

			// Set up a consumer first to capture the notes
			go func() {
				for note := range enexFile.NoteChannel {
					results = append(results, note)
				}
				wg.Done()
			}()

			// Start the producer
			err := enexFile.ReadFromFile()
			if err != tc.ExpectedError {
				t.Errorf("Expected error: %v, got: %v", tc.ExpectedError, err)
			}

			// Wait for all notes to be processed
			wg.Wait()

			// evaluate results
			if len(results) != tc.ExpectedNotes {
				t.Errorf("Expected %d notes, got %d", tc.ExpectedNotes, len(results))
			}
		})
	}
}
