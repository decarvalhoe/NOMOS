package com.example.demo.dto;

public record OrderResponse(
        Long id,
        String customerName,
        String status
) {}
