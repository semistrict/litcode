# Java Hello

## The Hello class

```java file=src/java/Hello.java lines=6-16
public class Hello {
    private String name;

    public Hello(String name) {
        this.name = name;
    }

    public String greet() {
        return "Hello, " + name + "!";
    }
}
```
