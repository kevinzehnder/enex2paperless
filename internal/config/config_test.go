package config

import (
	"testing"
)

func TestConfiguration(t *testing.T) {

	_, err := GetConfig()
	if err != nil {
		t.Errorf("configuration error: %s", err)
	}

}
