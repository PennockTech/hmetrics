// Copyright © 2020 Pennock Tech, LLC.
// All rights reserved, except as granted under license.
// Licensed per file LICENSE.txt

package hmetrics

import (
	"fmt"
)

func (e HTTPFailureError) Error() string {
	return fmt.Sprintf("http: got %q instead of %d from: %q (%s)",
		e.ActualResponseCode,
		e.ExpectedResponseCode,
		e.URL,
		e.Comment,
	)
}
