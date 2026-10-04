package backoffretry

import (
	"github.com/mostafakhairy0305-dot/singleton/internal/ports"
)

var _ ports.Retrier[int] = (*Retrier[int])(nil)
