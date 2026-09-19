
CREATE TABLE `pump_amm_info`
(
    `id`                               BIGINT                                                        NOT NULL AUTO_INCREMENT COMMENT 'Primary Key',
    `pool_account`                     VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Pool account address',
    `global_config_account`            VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Global configuration account address',
    `base_mint_account`                VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Base mint address',
    `quote_mint_account`               VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Quote mint address',
    `pool_base_token_account`          VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Pool''s base token account',
    `pool_quote_token_account`         VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Pool''s quote token account',
    `protocol_fee_recipient_account`   VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Protocol fee recipient account',
    `protocol_fee_recipient_token_account` VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Protocol fee recipient token account',
    `base_token_program`                VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Base token program address',
    `quote_token_program`               VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Quote token program address',
    `system_program`                    VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'System program address',
    `associated_token_program`          VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Associated token program address',
    `event_authority_account`           VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Event authority account',
    `program_account`                   VARCHAR(256) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL COMMENT 'Program account address',
    `coin_creator_vault_ata_account`    varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'Coin creator vault ata account',
    `coin_creator_vault_authority_account` varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL DEFAULT '' COMMENT 'Coin creator vault authority account',
    `created_at`                        TIMESTAMP                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp',
    `updated_at`                        TIMESTAMP                                                     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Update timestamp',
    `deleted_at`                        TIMESTAMP                                                     NULL DEFAULT NULL COMMENT 'Soft delete timestamp',
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_pool_account` (`pool_account`)  -- 添加唯一索引到 pool_account
) ENGINE = InnoDB
DEFAULT CHARSET = utf8mb4
COLLATE = utf8mb4_general_ci COMMENT ='Pump AMM Information';
