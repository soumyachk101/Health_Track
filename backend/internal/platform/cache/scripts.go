package cache

// RotateScript atomically rotates one opaque refresh token of a family
// (ADR-0001). The caller resolves family first with `GET rt_fam:<h0>` and
// passes every key it may write, so replication is key-correct.
//
// Keys the script touches:
//
//	`rt:<h>`             live marker for the CURRENT token hash of a family
//	`rt_fam:<h>`         hash -> family map (survives a rotation so a replayed
//	                      already-consumed token can still be attributed)
//	`rt_family:<f>`      family record: "uid|hcur"
//	`rt_family:<f>:rev` death flag for the whole family
//
// KEYS[1] = "rt:<h0>"             the presented token's live marker
// KEYS[2] = "rt_fam:<h0>"        the presented token's family map
// KEYS[3] = "rt_family:<f>"     the family record
// KEYS[4] = "rt_family:<f>:rev" the family death flag
// KEYS[5] = "rt:<h1>"           the successor's live marker
// KEYS[6] = "rt_fam:<h1>"       the successor's family map
// ARGV[1] = TTL in seconds
// ARGV[2] = h1, the successor token's hash
//
// Returns:
//   - "ok"              h0 was current: rotated, successor registered.
//   - "reused:<uid>"    h0 was already consumed (or another hash is
//     current): replay detected, the whole family is revoked atomically.
//   - "dead"            the family record or flag is already gone (killed by
//     an earlier replay, a logout, or TTL expiry): a plain session error.
const RotateScript = `
if redis.call("EXISTS", KEYS[4]) == 1 then
  return "dead"
end
local rec = redis.call("GET", KEYS[3])
if not rec then
  redis.call("SET", KEYS[4], "1", "EX", ARGV[1])
  return "dead"
end
local sep = string.find(rec, "|", 1, true)
local uid = string.sub(rec, 1, sep - 1)
local hcur = string.sub(rec, sep + 1)
local h0 = string.sub(KEYS[1], 4)
if hcur == h0 then
  redis.call("SET", KEYS[5], string.sub(KEYS[2], 7), "EX", ARGV[1])
  redis.call("SET", KEYS[6], string.sub(KEYS[2], 7), "EX", ARGV[1])
  redis.call("DEL", KEYS[1])
  redis.call("SET", KEYS[3], uid .. "|" .. ARGV[2], "EX", ARGV[1])
  return "ok"
end
redis.call("SET", KEYS[4], "1", "EX", ARGV[1])
redis.call("DEL", KEYS[3])
redis.call("DEL", KEYS[2])
return "reused:" .. uid
`
