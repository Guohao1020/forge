package com.shulex.forge.specs.service;

import com.shulex.forge.specs.infrastructure.entity.ProductDO;
import com.shulex.forge.specs.infrastructure.mapper.ProductMapper;
import com.shulex.forge.specs.service.dto.ProductDTO;
import com.shulex.forge.specs.service.impl.ProductServiceImpl;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class ProductServiceTest {

    @Mock
    private ProductMapper productMapper;

    private ProductServiceImpl productService;

    @BeforeEach
    void setUp() {
        productService = new ProductServiceImpl(productMapper);
    }

    @Test
    void findById_returnsProduct_whenFound() {
        ProductDO productDO = new ProductDO();
        productDO.setId(1L);
        productDO.setName("Widget");
        when(productMapper.selectById(1L)).thenReturn(productDO);

        ProductDTO result = productService.findById(1L);

        assertThat(result).isNotNull();
        assertThat(result.getId()).isEqualTo(1L);
        assertThat(result.getName()).isEqualTo("Widget");
    }

    @Test
    void findById_throwsException_whenNotFound() {
        when(productMapper.selectById(99L)).thenReturn(null);

        assertThatThrownBy(() -> productService.findById(99L))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("99");
    }
}
