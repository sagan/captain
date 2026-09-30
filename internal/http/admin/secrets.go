package admin

import (
	"github.com/zeptop-dev/bosun/pkg/spec"
	"github.com/zeptop-dev/captain/internal/domain"
)

func fillInboundSecrets(ib *domain.Inbound) {
	s := ib.Spec()
	spec.FillInboundSecrets(&s)
	ib.Settings = s
}
