package com.example.demo.service;

import com.example.demo.dto.OrderRequest;
import com.example.demo.dto.OrderResponse;
import com.example.demo.entity.Order;
import com.example.demo.repository.OrderRepository;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.stream.Collectors;

@Service
public class OrderService {

    private final OrderRepository orderRepository;

    public OrderService(OrderRepository orderRepository) {
        this.orderRepository = orderRepository;
    }

    public List<OrderResponse> findAll() {
        return orderRepository.findAll().stream()
                .map(this::toResponse)
                .collect(Collectors.toList());
    }

    public OrderResponse findById(Long id) {
        Order order = orderRepository.findById(id)
                .orElseThrow(() -> new RuntimeException("Order not found: " + id));
        return toResponse(order);
    }

    public OrderResponse create(OrderRequest request) {
        Order order = new Order();
        order.setCustomerName(request.customerName());
        order.setStatus(request.status());
        return toResponse(orderRepository.save(order));
    }

    public OrderResponse update(Long id, OrderRequest request) {
        Order order = orderRepository.findById(id)
                .orElseThrow(() -> new RuntimeException("Order not found: " + id));
        order.setCustomerName(request.customerName());
        order.setStatus(request.status());
        return toResponse(orderRepository.save(order));
    }

    public void delete(Long id) {
        orderRepository.deleteById(id);
    }

    private OrderResponse toResponse(Order order) {
        return new OrderResponse(order.getId(), order.getCustomerName(), order.getStatus());
    }
}
