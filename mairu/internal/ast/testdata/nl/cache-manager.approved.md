## evictStale (fn)

1. Assigns `0` to `evicted`
2. Assigns calling `Date.now` to `now`
3. Iterates over each `[key, entry]` in `cache`, If `now - entry.createdAt` is greater than `maxAge`, calling `cache.delete` with `key`; increments `evicted`
4. Returns `evicted`

## getOrSet (fn)

1. If `cache.has(key)`, Returns calling `cache.get` with `key`
2. Assigns calling `compute` to `value`
3. calling `cache.set` with `key`, `value`
4. Returns `value`

## warmUp (fn)

1. Iterates over each `key` in `keys`, If `cache.has(key)` is falsy, Assigns calling `loader` with `key` to `value`; calling `cache.set` with `key`, `value`
