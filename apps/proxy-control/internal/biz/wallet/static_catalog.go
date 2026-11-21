package wallet

import (
	"fmt"
	"strconv"
	"strings"
)

func validStaticCatalog(shareMode string, purposeID int32) bool {
	switch shareMode {
	case "", "exclusive":
		return purposeID == 0
	case "shared":
		return purposeID == 0 || purposeID == 8 || purposeID == 9 || purposeID == 10
	default:
		return false
	}
}

func (p StaticPurchaseRequest) OfferReference(nodeID int64) string {
	if p.ShareMode == "shared" {
		return fmt.Sprintf("%d/%s/shared/%d", nodeID, p.Term, p.PurposeID)
	}
	return fmt.Sprintf("%d/%s", nodeID, p.Term)
}

func ParseStaticPurchaseReference(reference string) (nodeID int64, shareMode string, purposeID int32, ok bool) {
	parts := strings.Split(reference, "/")
	if len(parts) != 2 && len(parts) != 4 {
		return 0, "", 0, false
	}
	nodeID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || nodeID <= 0 || parts[1] == "" {
		return 0, "", 0, false
	}
	if len(parts) == 2 {
		return nodeID, "", 0, true
	}
	purpose, err := strconv.ParseInt(parts[3], 10, 32)
	if err != nil || parts[2] != "shared" || !validStaticCatalog(parts[2], int32(purpose)) {
		return 0, "", 0, false
	}
	return nodeID, parts[2], int32(purpose), true
}
