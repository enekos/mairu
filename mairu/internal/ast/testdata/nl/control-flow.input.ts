export function exampleIfElse(x: number) {
  if (x > 10) {
    console.log("greater");
  } else if (x === 10) {
    console.log("equal");
  } else {
    console.log("less");
  }
}

export function exampleSwitch(val: string) {
  switch (val) {
    case "A":
      console.log("A");
      break;
    case "B":
      return 1;
    default:
      throw new Error("unknown");
  }
}

export function exampleLoops() {
  for (let i = 0; i < 5; i++) {
    console.log(i);
  }
  
  let j = 0;
  while (j < 5) {
    j++;
  }
  
  for (const k of [1, 2, 3]) {
    console.log(k);
  }
}
