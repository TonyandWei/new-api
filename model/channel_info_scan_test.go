package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestChannelInfoScan(t *testing.T) {
	t.Run("accepts string json", func(t *testing.T) {
		var info ChannelInfo
		err := info.Scan(`{"is_multi_key":true,"multi_key_size":2,"multi_key_status_list":{"1":2},"multi_key_disabled_reason":{"1":"quota"},"multi_key_disabled_time":{"1":123},"multi_key_polling_index":1,"multi_key_mode":"random"}`)
		require.NoError(t, err)
		require.True(t, info.IsMultiKey)
		require.Equal(t, 2, info.MultiKeySize)
		require.Equal(t, 2, info.MultiKeyStatusList[1])
		require.Equal(t, "quota", info.MultiKeyDisabledReason[1])
		require.EqualValues(t, 123, info.MultiKeyDisabledTime[1])
		require.Equal(t, 1, info.MultiKeyPollingIndex)
		require.Equal(t, constant.MultiKeyModeRandom, info.MultiKeyMode)
	})

	t.Run("accepts byte json", func(t *testing.T) {
		var info ChannelInfo
		err := info.Scan([]byte(`{"is_multi_key":false,"multi_key_size":0,"multi_key_polling_index":0,"multi_key_mode":""}`))
		require.NoError(t, err)
		require.False(t, info.IsMultiKey)
		require.Equal(t, 0, info.MultiKeySize)
	})

	t.Run("treats nil and empty as zero value", func(t *testing.T) {
		info := ChannelInfo{IsMultiKey: true, MultiKeySize: 3}
		require.NoError(t, info.Scan(nil))
		require.Equal(t, ChannelInfo{}, info)

		info = ChannelInfo{IsMultiKey: true, MultiKeySize: 3}
		require.NoError(t, info.Scan("   "))
		require.Equal(t, ChannelInfo{}, info)

		info = ChannelInfo{IsMultiKey: true, MultiKeySize: 3}
		require.NoError(t, info.Scan([]byte("null")))
		require.Equal(t, ChannelInfo{}, info)
	})
}
