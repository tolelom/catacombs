package store

import "errors"

var (
	errNotEnoughSouls = errors.New("not enough souls")
	errAtMaxLevel     = errors.New("upgrade at max level")
	errMissingUpgrade = errors.New("unknown upgrade id")
	errBucketMissing  = errors.New("bolt bucket missing")
)

var ErrNotEnoughSouls = errNotEnoughSouls

var ErrAtMaxLevel = errAtMaxLevel

var ErrMissingUpgrade = errMissingUpgrade
