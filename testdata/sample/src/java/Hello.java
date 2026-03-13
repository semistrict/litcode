package com.example;

import java.util.List;

// A simple greeting class.
public class Hello {
    private String name;

    public Hello(String name) {
        this.name = name;
    }

    public String greet() {
        return "Hello, " + name + "!";
    }
}
