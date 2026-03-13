# Python Calculator

## Basic operations

Addition and multiplication:

```py file=src/py/calculator.py lines=6-10
def add(a: float, b: float) -> float:
    return a + b

def multiply(a: float, b: float) -> float:
    return a * b
```

## Hypotenuse

Uses the Pythagorean theorem:

```py file=src/py/calculator.py lines=12-13
def hypotenuse(a: float, b: float) -> float:
    return math.sqrt(a * a + b * b)
```
