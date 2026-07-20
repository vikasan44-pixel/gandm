import unittest

from main import Limits, MatchRequest, Radii, ValidationError, match, parse_match_request


def cargo(identifier: str, client: str, lng: float) -> dict:
    return {
        "id": identifier, "client_id": client,
        "origin": {"lat": 43.25, "lng": lng, "country": "KZ"},
        "destination": {"lat": 44.0, "lng": lng, "country": "kz"},
        "volume_m3": 10, "weight_kg": 1000,
    }


def request(items: list[dict]) -> MatchRequest:
    return parse_match_request({
        "requests": items,
        "limits": {"max_volume_m3": 100, "max_weight_kg": 10000},
        "radii": {"cn_km": 100, "kz_km": 40},
    })


class MatchingTests(unittest.TestCase):
    def test_rejects_invalid_coordinates_and_loads(self):
        invalid = cargo("a", "c1", 76)
        invalid["origin"]["lat"] = 91
        with self.assertRaises(ValidationError):
            request([invalid])
        negative = cargo("a", "c1", 76)
        negative["weight_kg"] = -1
        with self.assertRaises(ValidationError):
            request([negative])

    def test_group_requires_pairwise_route_match(self):
        result = match(request([
            cargo("seed", "c1", 76.0), cargo("west", "c2", 75.6), cargo("east", "c3", 76.4),
        ]))
        self.assertEqual(len(result), 1)
        self.assertEqual(len(result[0]), 2)

    def test_same_client_is_not_grouped(self):
        self.assertEqual(match(request([cargo("a", "same", 76), cargo("b", "same", 76.01)])), [])

    def test_duplicate_ids_are_rejected(self):
        with self.assertRaises(ValidationError):
            request([cargo("same", "a", 76), cargo("same", "b", 76.01)])


if __name__ == "__main__":
    unittest.main()
