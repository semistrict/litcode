use std::collections::HashMap;

/// Counts the frequency of each word in the input.
fn word_count(text: &str) -> HashMap<&str, usize> {
    let mut counts = HashMap::new();
    for word in text.split_whitespace() {
        *counts.entry(word).or_insert(0) += 1;
    }
    counts
}

/// Joins words with a separator.
fn join_words(words: &[&str], sep: &str) -> String {
    words.join(sep)
}
