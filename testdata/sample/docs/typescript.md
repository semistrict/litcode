# TypeScript Greeter

## The Greeter class

The `Greeter` class wraps a name and produces greetings:

```ts file=src/ts/greeter.ts lines=4-14
export class Greeter {
  private name: string;

  constructor(name: string) {
    this.name = name;
  }

  greet(): string {
    return `Hello, ${this.name}!`;
  }
}
```
