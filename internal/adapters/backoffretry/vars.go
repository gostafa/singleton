// Gostafa 2026.
// SPDX-License-Identifier: Apache-2.0.

package backoffretry

import (
	"github.com/gostafa/singleton/internal/ports"
)

var _ ports.Retrier[int] = (*Retrier[int])(nil)
