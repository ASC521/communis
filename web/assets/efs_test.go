package assets_test

import (
	"regexp"
	"testing"

	"github.com/ASC521/communis/web/assets"
)

func TestEtagsCalculation(t *testing.T) {
	etags, err := assets.ComputeStaticFilesEtags()
	if err != nil {
		t.Errorf("failed to compute etags map: %e", err)
	}

	re := regexp.MustCompile(`W\/"[a-z0-9]{32}"`)
	for filePath, hash := range etags {
		if !re.Match([]byte(hash)) {
			t.Errorf("computed etag for %s does not match expected pattern", filePath)
		}
	}
}
