import math
from typing import List

# A simple calculator module.

def add(a: float, b: float) -> float:
    return a + b

def multiply(a: float, b: float) -> float:
    return a * b

def hypotenuse(a: float, b: float) -> float:
    return math.sqrt(a * a + b * b)
