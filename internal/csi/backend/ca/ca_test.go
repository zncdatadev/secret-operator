package ca

import (
	"encoding/json"
	"testing"
)

type Foo struct {
	Name string `json:"foo.example.com/name"`
	Age  int    `json:"foo.example.com/age"`
}

func TestFoo(t *testing.T) {

	mapData := map[string]interface{}{
		"foo.example.com/name": "John",
		// "foo.example.com/age":  30,
		"xxx": 111,
	}

	jsonBytes, err := json.Marshal(mapData)

	if err != nil {
		t.Fatal(err)
	}

	foo := &Foo{}

	// Unmarshal map data to struct
	err = json.Unmarshal(jsonBytes, foo)
	if err != nil {
		t.Fatal(err)
	}

}
