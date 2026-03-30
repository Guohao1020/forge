package com.example;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class ctrl {

    @Autowired
    private CategoryService s;

    // gets stuff
    @GetMapping("/cat/{x}")
    public Object doIt(@PathVariable Long x) {
        return s.get(x);
    }
}
