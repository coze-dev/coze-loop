package benefit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNoopBenefitService(t *testing.T) {
	svc := NewNoopBenefitService()
	assert.NotNil(t, svc)
}

func TestNoopBenefitServiceImpl_GetTraceBenefitSource(t *testing.T) {
	var svc IBenefitService = NoopBenefitServiceImpl{}
	res, err := svc.GetTraceBenefitSource(context.Background(), nil)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, int64(10), res.Source)
}

func TestNoopBenefitServiceImpl_CheckTraceBenefit(t *testing.T) {
	var svc IBenefitService = NoopBenefitServiceImpl{}
	res, err := svc.CheckTraceBenefit(context.Background(), &CheckTraceBenefitParams{})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.AccountAvailable)
	assert.True(t, res.IsEnough)
	assert.Equal(t, int64(365), res.StorageDuration)
	assert.Equal(t, -1, res.WhichIsEnough)
}

