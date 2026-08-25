package storage

import "time"

// domainTime keeps the sealing API explicit while avoiding nullable timestamps at call sites.
type domainTime struct{ Time time.Time }

func TimeValue(t time.Time) domainTime { return domainTime{Time: t} }
