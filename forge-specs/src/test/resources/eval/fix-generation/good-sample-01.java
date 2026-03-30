package com.shulex.forge.specs.service.impl;

import com.shulex.forge.specs.infrastructure.mapper.InventoryMapper;
import com.shulex.forge.specs.service.InventoryService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

@Service
public class InventoryServiceImpl implements InventoryService {

    private static final Logger log = LoggerFactory.getLogger(InventoryServiceImpl.class);

    private final InventoryMapper inventoryMapper;

    public InventoryServiceImpl(InventoryMapper inventoryMapper) {
        this.inventoryMapper = inventoryMapper;
    }

    @Override
    public int getStock(Long productId) {
        log.debug("Querying stock for productId: {}", productId);
        Integer stock = inventoryMapper.selectStockByProductId(productId);
        if (stock == null) {
            log.warn("No inventory record found for productId: {}", productId);
            return 0;
        }
        return stock;
    }
}
