package cache

// RotateScript atomically rotates one refresh token of a family in Redis.
//
//	KEYS[1] = "rt:<sha256-of-presented-token>"
//	KEYS[2] = "rt_family:<family_id>"
//	ARGV[1] = sha256 of the successor token
//	ARGV[2] = TTL for the new entry, in seconds
//
// Semantics:
//   - old token present  -> replace it (del + set new), extend the family,
//     return "ok".
//   - old token ABSENT   -> it was already consumed: this is a replay. Kill
//     the whole family (del + flag), return "reused".
//
// A replayed refresh token therefore revokes every still-live token of its
// family in the same atomic step.
const RotateScript = `
local present = redis.call("GET", KEYS[1])
if not present then
  redis.call("DEL", KEYS[2])
  redis.call("SET", KEYS[2] .. ":reused", "1", "EX", ARGV[2])
  return "reused"
end
redis.call("DEL", KEYS[1])
redis.call("SET", "rt:" .. ARGV[1], KEYS[2], "EX", ARGV[2])
redis.call("EXPIRE", KEYS[2], tonumber(ARGV[2]))
return "ok"
`

// ClaimOtpScript claims one hashed OTP/reset token exactly once.
//
//	KEYS[1] = "otp:<sha256-of-token>:<purpose>"
//	ARGV[1] = TTL, in seconds
//
// Returns "claimed" when the key was free and is now held, "taken" when a
// concurrent consumer already won. The claim doubles as the expiry marker:
// the key disappears after ARGV[1] seconds, so a late replay is a no-op.
const ClaimOtpScript = `
if redis.call("EXISTS", KEYS[1]) == 1 then
  return "taken"
end
redis.call("SET", KEYS[1], "1", "EX", tonumber(ARGV[1]))
return "claimed"
`
