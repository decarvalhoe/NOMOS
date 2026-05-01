package com.example.demo.dto;

public record ProductResponse(
        Long id,
        String name,
        String categoryCode,
        String categoryLabel
) {}
