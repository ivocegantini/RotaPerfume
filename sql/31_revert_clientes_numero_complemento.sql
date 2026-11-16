-- ============================================================
-- REVERT: desfaz sql/31 (ECOM-09) | Data: 2026-11-16 | DataBrain (🌸)
-- Remove numero e complemento (os valores gravados se perdem).
-- ============================================================

SET NAMES utf8mb4;

ALTER TABLE `clientes` DROP COLUMN IF EXISTS `complemento`, DROP COLUMN IF EXISTS `numero`;
ALTER TABLE `clientes` MODIFY `endereco` VARCHAR(255) NULL DEFAULT NULL COMMENT 'Logradouro, número e complemento';
