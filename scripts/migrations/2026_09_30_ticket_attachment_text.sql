-- Ticket and ticket response attachments can store up to 5 file URLs.
-- Widen the columns so a JSON list of public URLs fits.

-- migrate:up
ALTER TABLE `tickets`
  MODIFY `attachment` TEXT NULL;

ALTER TABLE `ticket_responses`
  MODIFY `attachment` TEXT NULL;

-- migrate:down
ALTER TABLE `tickets`
  MODIFY `attachment` VARCHAR(191) NULL;

ALTER TABLE `ticket_responses`
  MODIFY `attachment` VARCHAR(191) NULL;
