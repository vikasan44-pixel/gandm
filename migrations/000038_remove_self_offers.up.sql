-- Самоотклик не является реальным рыночным предложением. Удаляем записи,
-- которые могли появиться до серверной проверки владельца заявки.

DELETE FROM offers offer
USING cargo_requests cargo
WHERE offer.cargo_request_id = cargo.id
  AND offer.participant_id = cargo.client_id;

DELETE FROM offers offer
USING consolidated_requests consolidated
WHERE offer.consolidated_request_id = consolidated.id
  AND EXISTS (
      SELECT 1
      FROM cargo_requests cargo
      WHERE cargo.client_id = offer.participant_id
        AND consolidated.member_request_ids @> to_jsonb(cargo.id::text)
  );

