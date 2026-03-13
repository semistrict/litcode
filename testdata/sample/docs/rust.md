# Rust Word Utilities

## Word count

```rust file=src/rs/lib.rs lines=4-10
fn word_count(text: &str) -> HashMap<&str, usize> {
    let mut counts = HashMap::new();
    for word in text.split_whitespace() {
        *counts.entry(word).or_insert(0) += 1;
    }
    counts
}
```

## Join words

```rust file=src/rs/lib.rs lines=13-15
fn join_words(words: &[&str], sep: &str) -> String {
    words.join(sep)
}
```
