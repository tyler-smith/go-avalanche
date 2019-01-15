CREATE TABLE `users` (
  `id` INT(11) auto_increment PRIMARY KEY,
  `name` VARCHAR(255) NOT NULL,
  `key` VARCHAR(255) NOT NULL,

  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `deleted_at` TIMESTAMP NULL DEFAULT NULL
);

CREATE TABLE `participants` (
  -- `id` INT(11) auto_increment PRIMARY KEY,
  `user_id` INT(11) NOT NULL,
  `key` VARCHAR(255) NOT NULL,
  `active` BOOLEAN NOT NULL DEFAULT FALSE,

  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `active_at` TIMESTAMP NULL DEFAULT NULL,

  PRIMARY KEY (`key`)
  -- UNIQUE INDEX `idx_key` (`key`)
);

CREATE TABLE `transactions` (
  `id` CHAR(64) NOT NULL,

  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `confirmed_at` TIMESTAMP NULL DEFAULT NULL,

  PRIMARY KEY (`id`)
);

CREATE TABLE `vote_records` (
  `id` INT(11) auto_increment PRIMARY KEY,
  `participant_id` INT(11) NOT NULL,
  `tx_id` CHAR(64) NOT NULL,

  `query_count` INT(11) NOT NULL DEFAULT 0,
  `initialized_accepted` BOOLEAN NOT NULL,
  `finalized_accepted` BOOLEAN NOT NULL DEFAULT FALSE,

  `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` TIMESTAMP NOT NULL DEFAULT '1970-01-01 00:00:01',
  `finalized_at` TIMESTAMP NULL DEFAULT NULL,

  UNIQUE INDEX `idx_tx_id_participant_id` (`tx_Id`, `participant_id`)
);
