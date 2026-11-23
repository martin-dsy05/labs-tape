package app

import "testing"

func TestRunHelpReturnsSuccess(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}, {"images", "--help"}} {
		t.Run(args[0]+"_help", func(t *testing.T) {
			if code := run(args); code != 0 {
				t.Fatalf("help should succeed, got exit code %d", code)
			}
		})
	}
}

func TestRunInvalidOptionReturnsFailure(t *testing.T) {
	if code := run([]string{"--not-a-tape-option"}); code != 1 {
		t.Fatalf("invalid option should fail, got exit code %d", code)
	}
}
