package redisbridge

type scriptID uint8

const (
	scriptInvalid scriptID = iota
	scriptChallengeRate
	scriptChallengeBegin
	scriptChallengeCommit
	scriptChallengeAbort
	scriptChallengeReadVerifier
	scriptChallengeCompleteVerify
)

func (id scriptID) valid() bool {
	return id >= scriptChallengeRate && id <= scriptChallengeCompleteVerify
}

func ChallengeRateScript() Script           { return Script{id: scriptChallengeRate} }
func ChallengeBeginScript() Script          { return Script{id: scriptChallengeBegin} }
func ChallengeCommitScript() Script         { return Script{id: scriptChallengeCommit} }
func ChallengeAbortScript() Script          { return Script{id: scriptChallengeAbort} }
func ChallengeReadVerifierScript() Script   { return Script{id: scriptChallengeReadVerifier} }
func ChallengeCompleteVerifyScript() Script { return Script{id: scriptChallengeCompleteVerify} }

func scriptSource(id scriptID) string {
	switch id {
	case scriptChallengeRate:
		return challengeRateLua
	case scriptChallengeBegin:
		return challengeBeginLua
	case scriptChallengeCommit:
		return challengeCommitLua
	case scriptChallengeAbort:
		return challengeAbortLua
	case scriptChallengeReadVerifier:
		return challengeReadVerifierLua
	case scriptChallengeCompleteVerify:
		return challengeCompleteVerifyLua
	default:
		return ""
	}
}

const challengeRateLua = `
local caller = KEYS[1]
local global = KEYS[2]
local operation_id = ARGV[1]
local caller_window = tonumber(ARGV[2])
local caller_limit = tonumber(ARGV[3])
local global_window = tonumber(ARGV[4])
local global_limit = tonumber(ARGV[5])
local grace = tonumber(ARGV[6])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', caller, '-inf', now - caller_window)
redis.call('ZREMRANGEBYSCORE', global, '-inf', now - global_window)
local caller_replay = redis.call('ZSCORE', caller, operation_id)
local global_replay = redis.call('ZSCORE', global, operation_id)
if caller_replay and global_replay then return 'OK' end
if caller_replay or global_replay then return 'INCONSISTENT' end
if tonumber(redis.call('ZCARD', caller)) >= caller_limit then return 'CALLER' end
if tonumber(redis.call('ZCARD', global)) >= global_limit then return 'GLOBAL' end
redis.call('ZADD', caller, now, operation_id)
redis.call('ZADD', global, now, operation_id)
redis.call('PEXPIRE', caller, math.max(1, caller_window + grace))
redis.call('PEXPIRE', global, math.max(1, global_window + grace))
return 'OK'
`

const challengeBeginLua = `
local state = KEYS[1]
local quota = KEYS[2]
local issue_id = ARGV[1]
local max_issues = tonumber(ARGV[2])
local pending_ttl = tonumber(ARGV[5])
local challenge_ttl = tonumber(ARGV[6])
local cooldown_ttl = tonumber(ARGV[7])
local quota_window = tonumber(ARGV[8])
local grace = tonumber(ARGV[9])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)
local pending_id = redis.call('HGET', state, 'pending_id')
if pending_id then
  local pending_lease = tonumber(redis.call('HGET', state, 'pending_lease_until') or '0')
  if pending_id == issue_id and pending_lease > now then return 'OK' end
  if pending_lease > now then return 'PENDING' end
  redis.call('HDEL', state, 'pending_id', 'pending_digest', 'pending_pepper', 'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
end
local cooldown = tonumber(redis.call('HGET', state, 'cooldown_until') or '0')
if cooldown > now then return 'COOLDOWN' end
redis.call('ZREMRANGEBYSCORE', quota, '-inf', now - quota_window)
if tonumber(redis.call('ZCARD', quota)) >= max_issues then return 'QUOTA' end
redis.call('ZADD', quota, now, issue_id)
redis.call('HSET', state, 'pending_id', issue_id, 'pending_digest', ARGV[3], 'pending_pepper', ARGV[4], 'pending_lease_until', now + pending_ttl, 'pending_ttl_ms', challenge_ttl, 'pending_cooldown_ms', cooldown_ttl)
local active_expires = tonumber(redis.call('HGET', state, 'active_expires_at') or '0')
local state_deadline = math.max(active_expires, now + pending_ttl, cooldown)
redis.call('PEXPIRE', state, math.max(1, state_deadline - now + grace))
redis.call('PEXPIRE', quota, math.max(1, quota_window + grace))
return 'OK'
`

const challengeCommitLua = `
local state = KEYS[1]
local ops = KEYS[2]
local issue_id = ARGV[1]
local grace = tonumber(ARGV[2])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)
if redis.call('HGET', state, 'active_id') == issue_id then return 'OK' end
if redis.call('HGET', state, 'pending_id') ~= issue_id then return 'STALE' end
local lease = tonumber(redis.call('HGET', state, 'pending_lease_until') or '0')
if lease <= now then
  redis.call('HDEL', state, 'pending_id', 'pending_digest', 'pending_pepper', 'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
  return 'EXPIRED'
end
local digest = redis.call('HGET', state, 'pending_digest')
local pepper = redis.call('HGET', state, 'pending_pepper')
local expires = now + tonumber(redis.call('HGET', state, 'pending_ttl_ms'))
local cooldown = now + tonumber(redis.call('HGET', state, 'pending_cooldown_ms'))
redis.call('HSET', state, 'active_id', issue_id, 'active_digest', digest, 'active_pepper', pepper, 'active_expires_at', expires, 'active_attempts', 0, 'cooldown_until', cooldown)
redis.call('HDEL', state, 'pending_id', 'pending_digest', 'pending_pepper', 'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
redis.call('DEL', ops)
local deadline = math.max(expires, cooldown)
redis.call('PEXPIRE', state, math.max(1, deadline - now + grace))
return 'OK'
`

const challengeAbortLua = `
local state = KEYS[1]
local issue_id = ARGV[1]
if redis.call('HGET', state, 'last_abort_id') == issue_id then return 'OK' end
if redis.call('HGET', state, 'pending_id') ~= issue_id then return 'STALE' end
redis.call('HDEL', state, 'pending_id', 'pending_digest', 'pending_pepper', 'pending_lease_until', 'pending_ttl_ms', 'pending_cooldown_ms')
redis.call('HSET', state, 'last_abort_id', issue_id)
return 'OK'
`

const challengeReadVerifierLua = `
return {
  redis.call('HGET', KEYS[1], 'active_id'),
  redis.call('HGET', KEYS[1], 'active_digest'),
  redis.call('HGET', KEYS[1], 'active_pepper')
}
`

const challengeCompleteVerifyLua = `
local state = KEYS[1]
local ops = KEYS[2]
local expected_id = ARGV[1]
local expected_digest = ARGV[2]
local operation_id = ARGV[3]
local max_attempts = tonumber(ARGV[4])
local matched = tonumber(ARGV[5])
local idempotency_ttl = tonumber(ARGV[6])
local server_time = redis.call('TIME')
local now = tonumber(server_time[1]) * 1000 + math.floor(tonumber(server_time[2]) / 1000)
local replay = redis.call('HGET', ops, operation_id)
if replay then
  local separator = string.find(replay, '|', 1, true)
  if separator and string.sub(replay, 1, separator - 1) == expected_id then return string.sub(replay, separator + 1) end
  return 'STALE'
end
if redis.call('HGET', state, 'active_id') ~= expected_id or redis.call('HGET', state, 'active_digest') ~= expected_digest then return 'STALE' end
local function clear_active()
  redis.call('HDEL', state, 'active_id', 'active_digest', 'active_pepper', 'active_expires_at', 'active_attempts')
end
local function remember(result)
  redis.call('HSET', ops, operation_id, expected_id .. '|' .. result)
  redis.call('PEXPIRE', ops, idempotency_ttl)
  return result
end
local expires = tonumber(redis.call('HGET', state, 'active_expires_at') or '0')
if expires <= now then clear_active(); return remember('EXPIRED') end
if matched == 1 then clear_active(); return remember('SUCCESS') end
local attempts = tonumber(redis.call('HGET', state, 'active_attempts') or '0') + 1
if attempts >= max_attempts then clear_active(); return remember('LOCKED') end
redis.call('HSET', state, 'active_attempts', attempts)
return remember('INVALID')
`
