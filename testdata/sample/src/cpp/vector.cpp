#include <vector>
#include <algorithm>

// Returns the sum of all elements in a vector.
int sum(const std::vector<int>& v) {
    int total = 0;
    for (int x : v) {
        total += x;
    }
    return total;
}

// Returns the maximum element.
int max_element_val(const std::vector<int>& v) {
    return *std::max_element(v.begin(), v.end());
}
