package com.example.demo.service;

import com.example.demo.dto.ProductResponse;
import com.example.demo.entity.Product;
import com.example.demo.repository.ProductRepository;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;

@Service
public class ProductService {

    // HARDCODED CATALOG: should be externalized to configuration or database
    private static final Map<String, String> CATEGORY_LABELS = Map.of(
            "ELEC", "Electronics",
            "BOOK", "Books & Media",
            "FOOD", "Food & Beverages",
            "CLTH", "Clothing & Apparel"
    );

    private final ProductRepository productRepository;

    public ProductService(ProductRepository productRepository) {
        this.productRepository = productRepository;
    }

    public List<ProductResponse> findByCategory(String category) {
        List<Product> products = (category == null)
                ? productRepository.findAll()
                : productRepository.findByCategory(category);
        return products.stream().map(this::toResponse).collect(Collectors.toList());
    }

    public ProductResponse findById(Long id) {
        Product product = productRepository.findById(id)
                .orElseThrow(() -> new RuntimeException("Product not found: " + id));
        return toResponse(product);
    }

    public String getCategoryLabel(String code) {
        return CATEGORY_LABELS.getOrDefault(code, "Unknown");
    }

    private ProductResponse toResponse(Product p) {
        return new ProductResponse(p.getId(), p.getName(), p.getCategory(),
                getCategoryLabel(p.getCategory()));
    }
}
