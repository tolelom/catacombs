package store

import "errors"

var (
	errNotEnoughSouls = errors.New("not enough souls")
	errAtMaxLevel     = errors.New("upgrade at max level")
	errMissingUpgrade = errors.New("unknown upgrade id")
)

// ErrNotEnoughSouls is returned by BuyUpgrade when the balance is insufficient.
var ErrNotEnoughSouls = errNotEnoughSouls

// ErrAtMaxLevel is returned when trying to buy past max level.
var ErrAtMaxLevel = errAtMaxLevel

// ErrMissingUpgrade is returned for unknown upgrade IDs.
var ErrMissingUpgrade = errMissingUpgrade
