#!/usr/bin/env python

from __future__ import print_function

import copy
import json
import re
import requests
import string
import sys
import time

PRESERVED_KEYS = {}

# Borrowed from the 'six' library so we don't need it as a dependency
PY2 = sys.version_info[0] == 2
PY3 = sys.version_info[0] == 3
if PY3:
    string_types = (str,)
else:
    string_types = (basestring,)


def compare_structures(data1, data2, use_regex=False):
    if type(data1) is not type(data2):
        if use_regex and isinstance(data1, int) and isinstance(data2, string_types):
            if re.match(data2, str(data1)) is None:
                return False
            return True
        return False
    if isinstance(data1, list):
        if any(["METRONOMIKON_TEST_REQUIRE_ONE_LIST_MATCH_ONLY" in i for i in data1 + data2]):
            for idx1 in range(0, len(data1)):
                for idx2 in range(0, len(data2)):
                    if compare_structures(data1[idx1], data2[idx2], use_regex=use_regex):
                        return True
            return False
        if len(data1) != len(data2):
            return False
        data2 = copy.deepcopy(data2)
        for idx, var1 in enumerate(data1):
            # order in list of dicts doesn't matter
            if isinstance(var1, dict):
                matchFound = False
                for var2 in data2:
                    if compare_structures(var1, var2, use_regex=use_regex):
                        data2.remove(var2)
                        matchFound = True
                        break
                if not matchFound:
                    return False
            else:
                if not compare_structures(data1[idx], data2[idx], use_regex=use_regex):
                    return False
    elif isinstance(data2, dict):
        if sorted(data1.keys()) != sorted(data2.keys()):
            return False
        for key in data1:
            if not compare_structures(data1[key], data2[key], use_regex=use_regex):
                return False
    else:
        if use_regex and isinstance(data1, string_types):
            if re.match(data2, data1) is None:
                return False
            return True
        return (data1 == data2)
    return True


def perform_request(test_data, url):
    global PRESERVED_KEYS
    if test_data['method'] == 'GET':
        r = requests.get(url)
    elif test_data['method'] == 'PUT':
        r = requests.put(url)
    elif test_data['method'] == 'POST':
        r = requests.post(url)
    elif test_data['method'] == 'DELETE':
        r = requests.delete(url)
    else:
        print('Unsupported HTTP method: %s' % test_data['method'])
        return False

    if r.status_code != test_data['responseCode']:
        print('Got response code %d, expected %d' % (r.status_code, test_data['responseCode']))
        return False

    if 'responseText' in test_data:
        if r.text != test_data['responseText']:
            print('Got response text:\n\n%s\n\nexpected:\n\n%s' % (r.text, test_data['responseText']))
            return False
    elif 'responseJsonFile' in test_data:
        response_data = json.load(open('%s/%s' % (TEST_DIR, test_data['responseJsonFile'])))
        if not compare_structures(r.json(), response_data, use_regex=test_data.get('useRegex', False)):
            print('Got response JSON:\n\n%s\n\nexpected:\n\n%s' % (r.text, json.dumps(response_data)))
            return False
        if test_data.get('preserveKeys', False):
            PRESERVED_KEYS = r.json()
    return True


ENDPOINT = sys.argv[1]
TEST_DIR = sys.argv[2]

test_data = json.load(open('%s/metadata.json' % TEST_DIR))

if 'steps' in test_data:
    steps = test_data['steps']
else:
    steps = [test_data]

for step in steps:
    retries = step.get('retries', 0)

    print('Running test step "%s"...' % step['name'])

    url = '%s%s' % (ENDPOINT, string.Template(step['urlPath']).safe_substitute(PRESERVED_KEYS))

    print('Performing %s request against URL %s' % (step['method'], url))

    test_success = False
    for i in range(retries + 1):
        if perform_request(step, url):
            test_success = True
            break
        print("Attempt %s of %s failed" % (i+1, retries + 1))
        time.sleep(1)

    if test_success:
        continue
    else:
        sys.exit(1)
print('Test succeded')
