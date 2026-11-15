package integration

import (
	"encoding/json"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
	"github.com/DuanJinTaoToday/proxy/apps/proxy-control/integration/testfixture"
)

func TestRecorderPreservesUnmeteredEstablishedSOCKSConnections(t *testing.T) {
	f := setupUsage(t)
	var offset uint64
	var batch []*v1.RecorderLine
	for _, sample := range []struct {
		src, dst, message string
		input, output     int
	}{
		{"", "", "connection refused", 6, 7},
		{"", "", "i/o timeout", 8, 9},
		{"", "", "", 10, 11}, // policy denial with no err field
		{"127.0.0.1:3000", "127.0.0.1:3001", "", 10, 11},
		{"127.0.0.1:3000", "127.0.0.1:3001", "", 4, 5}, // successful connection, protocol bytes only
	} {
		raw, err := json.Marshal(map[string]any{
			"time": beforeMidnight, "network": "tcp", "clientID": "client-a", "src": sample.src, "dst": sample.dst, "err": sample.message,
			"inputBytes": sample.input, "outputBytes": sample.output, "inputBytesDelta": sample.input, "outputBytesDelta": sample.output,
		})
		if err != nil {
			t.Fatal(err)
		}
		end := offset + uint64(len(raw)) + 1
		batch = append(batch, &v1.RecorderLine{FileId: "sessions", FileOffset: offset, EndOffset: end, Line: string(raw)})
		offset = end
	}
	reply := submitResponse(t, f, "gateway-a", reportToken, batch...)
	testfixture.AssertStatus(t, reply, 503, "METERING_UNAVAILABLE")
	var cursors int64
	if err := f.db.Table("ingest_cursors").Count(&cursors).Error; err != nil || cursors != 0 {
		t.Fatalf("unmetered batch advanced the cursor: %d, %v", cursors, err)
	}
	// The failed connections can still be confirmed and replayed independently.
	if ack := submit(t, f, "gateway-a", reportToken, batch[:3]...); ack.GetNotBillable() != 3 {
		t.Fatalf("unexpected failed-connection dispositions: %v", ack)
	}
	if ack := submit(t, f, "gateway-a", reportToken, batch[:3]...); ack.GetDuplicates() != 3 {
		t.Fatalf("retry failed to deduplicate confirmed records: %v", ack)
	}
}
