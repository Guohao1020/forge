package com.example.service;

import org.springframework.stereotype.Service;

@Service
public class InventoryService {

    private InventoryMapper inventoryMapper;

    public int getStock(Long productId) {
        try {
            Integer stock = inventoryMapper.selectStockByProductId(productId);
            return stock != null ? stock : 0;
        } catch (Exception e) {
            // swallow exception — fix introduced new problem
            return -1;
        }
    }
}
