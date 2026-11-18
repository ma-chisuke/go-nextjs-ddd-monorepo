package events

import (
	_ "embed" // The YAML fixtures are files in testdata.
	"reflect"
	"testing"
)

// messagesAsyncAPI has messages in a channel and in components, with
// references inside the document, a cycle, an escaped name, and a reference
// to the network.
//
//go:embed testdata/messages.asyncapi.yaml
var messagesAsyncAPI string

// lookupYAML has an alias and a key with the characters that a JSON pointer
// escapes.
//
//go:embed testdata/lookup.yaml
var lookupYAML string

func TestLoadAsyncAPIFindsMessagesAndTheirFiles(t *testing.T) {
	doc, err := loadAsyncAPI(repository(map[string]string{fixtureAsyncAPI: messagesAsyncAPI}), fixtureAsyncAPI)
	if err != nil {
		t.Fatal(err)
	}
	want := []message{
		{name: "facility.RoomOpened.v1", line: 6, refs: []string{"contracts/events/facility/RoomOpened.v1.json"}},
		{name: "facility.RoomCleaned.v1", line: 14, refs: []string{"contracts/events/facility/RoomCleaned.v1.json"}},
		{name: "facility.RoomLooped.v1", line: 18, refs: []string{"contracts/events/facility/RoomLooped.v1.json"}},
		{name: "facility.RoomEscaped.v1", line: 22, refs: []string{"contracts/events/facility/RoomEscaped.v1.json"}},
		{name: "facility.RoomRemote.v1", line: 26},
	}
	if !reflect.DeepEqual(doc.messages, want) {
		t.Errorf("messages = %+v\nwant %+v", doc.messages, want)
	}
}

func TestLoadAsyncAPIOfAMissingDocument(t *testing.T) {
	doc, err := loadAsyncAPI(repository(nil), fixtureAsyncAPI)
	if err != nil || !doc.missing {
		t.Errorf("loadAsyncAPI() = %+v, %v; want a missing document", doc, err)
	}
}

func TestLookup(t *testing.T) {
	root, err := parseYAML([]byte(lookupYAML))
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"/a/1/b/c":     "found",
		"/d/c":         "found",
		"/e~1f~0g":     "found too",
		"/e%7E1f~0g":   "found too",
		"/a/2":         "",
		"/a/x":         "",
		"/a/0/b":       "",
		"no-slash":     "",
		"/%zz":         "",
		"/missing/c":   "",
		"/a/-1/b":      "",
		"/a/1/b/c/d":   "",
		"/e%7e1f%7E0g": "found too",
	}
	for ptr, want := range tests {
		got := ""
		if n := lookup(root, ptr); n != nil {
			got = n.Value
		}
		if got != want {
			t.Errorf("lookup(%q) = %q, want %q", ptr, got, want)
		}
	}
}
