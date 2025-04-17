package main

import "testing"

// TestIsNLQContentType verifies that various Content-Type header values
// are correctly recognized (or not) as application/nlq.
func TestIsNLQContentType(t *testing.T) {
   tests := []struct {
       ct   string
       want bool
   }{
       {"application/nlq", true},
       {"application/nlq; charset=utf-8", true},
       {"APPLICATION/NLQ", true},
       {"application/NLQ; param=val", true},
       {"application/json", false},
       {"application/nlq+xml", false},
       {"", false},
       {"invalid/md", false},
   }
   for _, tc := range tests {
       got := isNLQContentType(tc.ct)
       if got != tc.want {
           t.Errorf("isNLQContentType(%q) = %v; want %v", tc.ct, got, tc.want)
       }
   }
}