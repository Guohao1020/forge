package com.shulex.forge.specs.entrance.controller;

import com.shulex.forge.common.result.Result;
import com.shulex.forge.specs.entrance.vo.CategoryVO;
import com.shulex.forge.specs.service.CategoryService;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * REST controller for category management.
 *
 * <p>Provides endpoints to query product categories. All responses are
 * wrapped in {@link Result} to ensure a consistent API contract.</p>
 */
@RestController
@RequestMapping("/api/v1/categories")
public class CategoryController {

    private static final Logger log = LoggerFactory.getLogger(CategoryController.class);

    private final CategoryService categoryService;

    /**
     * Constructs a {@code CategoryController} with required dependencies.
     *
     * @param categoryService service layer for category operations; must not be {@code null}
     */
    public CategoryController(CategoryService categoryService) {
        this.categoryService = categoryService;
    }

    /**
     * Retrieves a single category by its unique identifier.
     *
     * @param id the category ID; must be a positive long value
     * @return {@link Result} containing the {@link CategoryVO}, or an error result if not found
     */
    @GetMapping("/{id}")
    public Result<CategoryVO> getCategory(@PathVariable Long id) {
        log.info("Request to get category with id: {}", id);
        CategoryVO category = categoryService.findById(id);
        return Result.success(category);
    }
}
