package main

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestReseedPseudoRandAndInBackground(t *testing.T) {
	ReseedPseudoRandAndInBackground()
}

func TestPrepareUtilitiesAndInBackground(t *testing.T) {
	PrepareUtilitiesAndInBackground()
}
