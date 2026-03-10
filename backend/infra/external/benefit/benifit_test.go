package benefit

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTraceBenefitSourceParams_JSONTags(t *testing.T) {
	b, err := json.Marshal(&GetTraceBenefitSourceParams{
		Tags:       map[string]string{"k1": "v1"},
		SystemTags: map[string]string{"k2": "v2"},
	})
	assert.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(b, &got)
	assert.NoError(t, err)
	assert.Contains(t, got, "source")
	assert.Contains(t, got, "connector_uid")
}

func TestCheckTraceBenefitParams_JSONTags(t *testing.T) {
	b, err := json.Marshal(&CheckTraceBenefitParams{
		Source:       2,
		ConnectorUID: "u",
		SpaceID:      3,
	})
	assert.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(b, &got)
	assert.NoError(t, err)
	assert.Equal(t, float64(2), got["source"])
	assert.Equal(t, "u", got["connector_uid"])
	assert.Equal(t, float64(3), got["space_id"])
}
