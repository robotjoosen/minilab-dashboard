package discovery_test

import (
	"testing"
	"time"

	"github.com/robotjoosen/minilab-dashboard/pkg/discovery"
)

func TestBrowseReturnsWithoutErrorWhenNoResponders(t *testing.T) {
	var found []discovery.Entry

	err := discovery.Browse("_minilab-agent-test-none._tcp", 200*time.Millisecond, func(e discovery.Entry) {
		found = append(found, e)
	})
	if err != nil {
		t.Fatalf("Browse() error = %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("expected no entries for a service name nobody advertises, got %+v", found)
	}
}
