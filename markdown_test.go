package main

import (
	"strings"
	"sync"
	"testing"
)

// quoted long-form events are rendered from several goroutines at once, see
// renderQuotesAsHTML, which crashed with concurrent map writes on heading ids.
func TestMdToHTMLConcurrent(t *testing.T) {
	md := "# heading\n\ntext\n\n## heading\n\nmore text\n"
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				out := mdToHTML(md, j%2 == 0)
				if !strings.Contains(out, "heading") {
					t.Errorf("unexpected output: %s", out)
					return
				}
			}
		}()
	}
	wg.Wait()
}
