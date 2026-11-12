package integration

import (
	"encoding/json"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

func TestRecorderOnlyMetersEstablishedSOCKSConnections(t *testing.T) {
	f := setupUsage(t)
	var offset uint64
	var batch []*v1.RecorderLine
	for _, sample := range []struct {
		src, dst, message string
		input, output     int
	}{
		{"127.0.0.1:3000", "127.0.0.1:3001", "", 10, 11},
		{"127.0.0.1:3000", "127.0.0.1:3001", "", 4, 5}, // successful connection, protocol bytes only
		{"", "", "connection refused", 6, 7},
		{"", "", "i/o timeout", 8, 9},
		{"", "", "", 10, 11}, // policy denial with no err field
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
	ack := submit(t, f, "gateway-a", reportToken, batch...)
	if ack.GetBillable() != 2 || ack.GetNotBillable() != 3 || ack.GetQuarantined() != 0 {
		t.Fatalf("unexpected disposition: %v", ack)
	}
	assertDays(t, daily(t, f, "alpha", f.alice), usageDay("2020-11-10", 14, 16, 30))
}
