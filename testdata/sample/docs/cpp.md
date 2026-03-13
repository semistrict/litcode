# C++ Vector Utilities

## Sum

```cpp file=src/cpp/vector.cpp lines=5-11
int sum(const std::vector<int>& v) {
    int total = 0;
    for (int x : v) {
        total += x;
    }
    return total;
}
```

## Max element

```cpp file=src/cpp/vector.cpp lines=14-16
int max_element_val(const std::vector<int>& v) {
    return *std::max_element(v.begin(), v.end());
}
```
