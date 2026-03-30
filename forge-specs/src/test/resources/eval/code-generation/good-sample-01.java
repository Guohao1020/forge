package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.common.result.Result;
import com.shulex.forge.specs.entrance.vo.OrderVO;
import com.shulex.forge.specs.service.OrderService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/v1/orders")
public class OrderController {

    private static final Logger log = LoggerFactory.getLogger(OrderController.class);

    private final OrderService orderService;

    public OrderController(OrderService orderService) {
        this.orderService = orderService;
    }

    @GetMapping("/{id}")
    public Result<OrderVO> getOrder(@PathVariable Long id) {
        log.info("Fetching order with id: {}", id);
        OrderVO order = orderService.findById(id);
        return Result.success(order);
    }
}
