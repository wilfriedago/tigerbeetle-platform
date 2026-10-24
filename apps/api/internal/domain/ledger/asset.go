package ledger

import (
	"fmt"
	"regexp"
	"strconv"
)

// MaxAssetScale bounds the number of decimal places an asset can declare. Amounts are integers in
// the smallest unit (ADR-0002), and TigerBeetle stores them as unsigned 128-bit numbers.
const MaxAssetScale = 18

var assetPattern = regexp.MustCompile(`^([A-Z][A-Z0-9]{1,15})(?:/(\d{1,2}))?$`)

// Asset is a currency or unit with its scale, written as "XOF/0" or "EUR/2" (ADR-0005). Each
// (ledger, asset) pair maps to its own TigerBeetle ledger number, so the scale is fixed once the
// asset is used and "EUR/2" and "EUR/3" are different assets.
type Asset struct {
	Code  string
	Scale uint8
}

// ParseAsset parses "CODE" or "CODE/SCALE". A missing scale means zero decimal places.
func ParseAsset(s string) (Asset, error) {
	m := assetPattern.FindStringSubmatch(s)
	if m == nil {
		return Asset{}, fmt.Errorf("%w: asset %q must look like XOF/0 or EUR/2", ErrInvalid, s)
	}
	asset := Asset{Code: m[1]}
	if m[2] != "" {
		scale, err := strconv.ParseUint(m[2], 10, 8)
		if err != nil || scale > MaxAssetScale {
			return Asset{}, fmt.Errorf("%w: asset scale must be between 0 and %d", ErrInvalid, MaxAssetScale)
		}
		asset.Scale = uint8(scale)
	}
	return asset, nil
}

// String renders the canonical form, always with the scale, so "EUR" and "EUR/0" compare equal.
func (a Asset) String() string {
	return a.Code + "/" + strconv.Itoa(int(a.Scale))
}
