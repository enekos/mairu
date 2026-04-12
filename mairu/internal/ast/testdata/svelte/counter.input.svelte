<script>
  import { onMount } from 'svelte'

  let count = 0

  function increment() {
    count += 1
  }

  function decrement() {
    count -= 1
  }

  function reset() {
    count = 0
  }

  onMount(() => {
    console.log('mounted')
  })
</script>

<button on:click={decrement}>-</button>
<span>{count}</span>
<button on:click={increment}>+</button>
<button on:click={reset}>Reset</button>
