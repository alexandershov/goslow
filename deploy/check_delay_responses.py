import time

import requests


EXPECTED_RESPONSE = '{"goslow": "response"}'


def main(domain):
    for delay in [1, 2]:
        url = 'http://{:d}.{}'.format(delay, domain)
        resp = responds_at_least_in(url, delay)
        assert resp.text == EXPECTED_RESPONSE, \
            "Expected {} to return {}, got {}".format(url, EXPECTED_RESPONSE, resp.text)



def responds_at_least_in(url ,delay):
    start = time.time()
    resp = requests.get(url, allow_redirects=False)
    end = time.time()
    duration = end - start
    resp.raise_for_status()
    assert duration > delay, "{} responded in {} seconds, should be at least {}".format(url, duration, delay)
    return resp


if __name__ == '__main__':
    main('goslow.link')
