package com.shulex.forge.specs.service.impl;

import com.shulex.forge.specs.infrastructure.entity.ProductDO;
import com.shulex.forge.specs.infrastructure.mapper.ProductMapper;
import com.shulex.forge.specs.service.ProductService;
import com.shulex.forge.specs.service.dto.ProductDTO;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;

@Service
public class ProductServiceImpl implements ProductService {

    private static final Logger log = LoggerFactory.getLogger(ProductServiceImpl.class);

    private final ProductMapper productMapper;

    public ProductServiceImpl(ProductMapper productMapper) {
        this.productMapper = productMapper;
    }

    @Override
    public ProductDTO findById(Long id) {
        log.info("Looking up product by id: {}", id);
        ProductDO productDO = productMapper.selectById(id);
        if (productDO == null) {
            log.warn("Product not found for id: {}", id);
            throw new IllegalArgumentException("Product not found: " + id);
        }
        return toDTO(productDO);
    }

    private ProductDTO toDTO(ProductDO productDO) {
        ProductDTO dto = new ProductDTO();
        dto.setId(productDO.getId());
        dto.setName(productDO.getName());
        return dto;
    }
}
