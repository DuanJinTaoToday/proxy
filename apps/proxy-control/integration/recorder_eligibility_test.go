package integration

import (
	"encoding/json"
	"testing"

	v1 "github.com/DuanJinTaoToday/proxy/api/proxycontrol/v1"
)

func TestRecorderMetersEstablishedSOCKSConnectionsOnce(t *testing.T) {
	f := setupUsage(t)
	execSQL(t, f.db, "INSERT INTO tenants(id,name,primary_account_name) VALUES ('alpha','Alpha','primary')")
	execSQL(t, f.db, "INSERT INTO tenant_balances(tenant_id,available_bytes,updated_at) VALUES ('alpha',1,now())")
	execSQL(t, f.db, "INSERT INTO proxy_subaccounts(client_id,tenant_id,name,password_ciphertext,active) VALUES ('client-a','alpha','acct',decode('00','hex'),false)")
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
	if ack := submit(t, f, "gateway-a", reportToken, batch...); ack.GetBillable() != 2 || ack.GetNotBillable() != 3 {
		t.Fatalf("unexpected dispositions: %v", ack)
	}
	if ack := submit(t, f, "gateway-a", reportToken, batch...); ack.GetDuplicates() != 5 || ack.GetBillable() != 0 {
		t.Fatalf("repeat charged again: %v", ack)
	}
	var measured struct{ AvailableBytes, UsedBytes int64 }
	if err := f.db.Raw("SELECT available_bytes,used_bytes FROM tenant_balances JOIN proxy_subaccounts USING(tenant_id) WHERE client_id='client-a'").Scan(&measured).Error; err != nil || measured.AvailableBytes != -29 || measured.UsedBytes != 30 {
		t.Fatalf("expected disabled account to accrue 30 bytes and tenant to owe 29: %+v, %v", measured, err)
	}
}
