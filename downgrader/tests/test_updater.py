import sys
import unittest
from pathlib import Path
from unittest.mock import MagicMock, patch

import requests

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import updater


class VersionComparisonTests(unittest.TestCase):
    def test_only_newer_release_is_an_update(self):
        self.assertTrue(updater.is_newer_version("v0.2.1", "v0.2.0"))
        self.assertFalse(updater.is_newer_version("v0.2.0", "v0.2.0"))
        self.assertFalse(updater.is_newer_version("v0.1.1", "v0.2.0"))

    def test_invalid_release_tag_is_not_an_update(self):
        self.assertFalse(updater.is_newer_version("latest", "v0.2.0"))


class InternetCheckTests(unittest.TestCase):
    def test_has_internet_succeeds_on_first_reachable_host(self):
        response = MagicMock()
        with patch("updater.requests.get", return_value=response) as get:
            self.assertTrue(updater.has_internet())
        get.assert_called_once()
        response.close.assert_called_once()

    def test_has_internet_tries_fallbacks_before_failing(self):
        with patch(
            "updater.requests.get",
            side_effect=requests.ConnectionError("unreachable"),
        ) as get:
            self.assertFalse(updater.has_internet())
        self.assertEqual(get.call_count, len(updater._CONNECTIVITY_URLS))

    def test_ssl_error_on_one_host_does_not_block_later_success(self):
        response = MagicMock()
        with patch(
            "updater.requests.get",
            side_effect=[
                requests.exceptions.SSLError("bad cert"),
                response,
            ],
        ) as get:
            self.assertTrue(updater.has_internet())
        self.assertEqual(get.call_count, 2)
        response.close.assert_called_once()


if __name__ == "__main__":
    unittest.main()
