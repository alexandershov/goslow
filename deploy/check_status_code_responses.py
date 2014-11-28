import random

import requests


EXPECTED_RESPONSE = '{"goslow": "response"}'


def main(min_status_code, max_status_code, domain):
    all_status_codes = range(min_status_code, max_status_code + 1)
    status_codes_to_test = [random.choice(all_status_codes) for _ in range(10)]
    for status_code in status_codes_to_test:
        url = 'http://{:d}.{}'.format(status_code, domain)
        resp = requests.get(url, allow_redirects=False)
        assert resp.text == EXPECTED_RESPONSE, \
            "Expected {} to return {}, got {}".format(url, EXPECTED_RESPONSE, resp.text)
        assert resp.status_code == status_code, \
            "Expected {} to return {:d} status code, got {:d}".format(url, status_code, resp.status_code)


if __name__ == '__main__':
    main(200, 599, 'goslow.link')
