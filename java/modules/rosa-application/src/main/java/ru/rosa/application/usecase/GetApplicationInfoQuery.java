package ru.rosa.application.usecase;

import ru.rosa.domain.model.ProductIdentity;

@FunctionalInterface
public interface GetApplicationInfoQuery {
    ProductIdentity execute();
}

