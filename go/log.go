// Guest-facing logging -- thin, level-specific wrappers over go-pdk's own
// Log(level, msg), which already calls Extism's built-in extism:host/env
// log_* imports directly (no natyv-core host function involved at all).
// Where these lines actually go is controlled entirely host-side, by
// conf.natyv.json's logging field -- see natyv-io/shared's Config.zig and
// natyv-io/core's Logging.zig for that half.
package natyv

import "github.com/extism/go-pdk"

func LogTrace(s string) { pdk.Log(pdk.LogTrace, s) }
func LogDebug(s string) { pdk.Log(pdk.LogDebug, s) }
func LogInfo(s string)  { pdk.Log(pdk.LogInfo, s) }
func LogWarn(s string)  { pdk.Log(pdk.LogWarn, s) }
func LogError(s string) { pdk.Log(pdk.LogError, s) }
