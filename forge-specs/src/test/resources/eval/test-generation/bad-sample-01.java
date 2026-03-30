package com.example;

import org.junit.jupiter.api.Test;

public class Test1 {

    @Test
    public void test() {
        ProductService s = new ProductService();
        Object result = s.findById(1L);
        // TODO: check result someday
        System.out.println(result);
    }

    @Test
    public void test2() {
        // no assertions
        new ProductService().findById(2L);
    }
}
