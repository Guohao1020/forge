package com.example.controller;

import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/orders")
public class order_controller {

    @Autowired
    private OrderService svc;

    @GetMapping("/{id}")
    public Object GetOrder(@PathVariable Long id) {
        System.out.println("Fetching order: " + id);
        return svc.findById(id);
    }
}
