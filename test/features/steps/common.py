import re
import requests
import time

BASE_URL = 'http://localhost:8080'


def send_api_request(context, method, path, data=None, headers={}, timeout=10, **kwargs):
    func = getattr(requests, method.lower())
    url = '%s/%s' % (BASE_URL, path)
    resp = func(url, json=data, headers=headers, timeout=timeout, **kwargs)
    return resp


def check_response_status(resp, expected_status=None):
    try:
        if expected_status is None:
            resp.raise_for_status()
        else:
            assert (resp.status_code == expected_status)
    except:
        print('Request headers:')
        print()
        print(resp.request.headers)
        print()
        if resp.request.body:
            print('Request body:')
            print()
            print(resp.request.body.decode())
            print()
        print('Response body:')
        print()
        print(resp.text)
        raise


def assert_expected_response(response, expected):
    try:
        assert response == expected
    except:
        print('Expected response: %s' % expected)
        print('Actual response: %s' % response)
        raise


def validate_response_content(response, expected):
    '''
    Recursively validate response content against expected value

    This function supports regex by using re.compile() in an expected value
    '''
    if isinstance(expected, dict):
        for key in expected:
            if key not in response:
                return False
            if not validate_response_content(response[key], expected[key]):
                return False
    elif isinstance(expected, list):
        for idx, item in enumerate(expected):
            if not validate_response_content(response[idx], expected[idx]):
                return False
    else:
        if isinstance(expected, re.Pattern):
            if expected.match(str(response)) is None:
                return False
        elif response != expected:
            return False

    return True


def build_expected_job_response(table):
    ret = []
    for row in table:
        cmd = "date; echo Hello from the Kubernetes cluster %s" % (row['name'])
        if row.get('command', None) is not None and row['command'].strip() != '':
            cmd = row['command']
        tmp = dict(
            id='%s.%s' % (row['namespace'], row['name']),
            run=dict(
                args=[
                    "/bin/sh",
                    "-c",
                    cmd,
                ],
                cpus=1,
                disk=5,
                docker=dict(
                    image="busybox",
                ),
                mem=512,
                placement=dict(),
                restart=dict(),
            ),
        )
        ret.append(tmp)
    return ret


def retry_step(max_attempts=10, sleep=0.2):
    '''
    Decorator function for retrying a step
    '''
    def retry_step_decorator(func):
        def retry_step_wrapper(*args, **kwargs):
            exc = None
            for attempt in range(1, max_attempts+1):
                if attempt > 1:
                    print('AUTO-RETRY STEP (attempt %d)' % attempt)
                try:
                    exc = None
                    return func(*args, **kwargs)
                except Exception as e:
                    exc = e
                    if sleep:
                        time.sleep(sleep)
            if exc is not None:
                print('AUTO-RETRY STEP FAILED (after %d attempts)' % max_attempts)
                raise exc
        return retry_step_wrapper
    return retry_step_decorator


def shared_check_job_history_response(response, table):
    expected = build_expected_job_response(table)
    for idx, row in enumerate(expected):
        exp = row
        job_id = exp['id']
        finished_runs_key = None
        if table[idx].get('failed', False):
            exp['history'] = dict(
                successCount=0,
                failureCount=re.compile(r'[1-9]+'),
                lastSuccessAt=None,
                lastFailureAt=re.compile(r'20.*'),
                successfulFinishedRuns=[],
            )
            finished_runs_key = 'failedFinishedRuns'
        else:
            exp['history'] = dict(
                successCount=re.compile(r'[1-9][0-9]*'),
                failureCount=0,
                lastSuccessAt=re.compile(r'20.*'),
                lastFailureAt=None,
                failedFinishedRuns=[],
            )
            finished_runs_key = 'successfulFinishedRuns'
        if not validate_response_content(response[idx], exp):
            raise Exception('Did not find expected job history response')
        # Look for one matching item under the appropriate finished runs key
        for item in response[idx]['history'][finished_runs_key]:
            item_exp = None
            if table[idx].get('failed', False):
                item_exp = dict(
                    id=re.compile(r'%s-[0-9]+' % job_id),
                    createdAt=re.compile(r'20.*'),
                    finishedAt=None,
                    tasks=[]
                )
            else:
                item_exp = dict(
                    id=re.compile(r'%s-[0-9]+' % job_id),
                    createdAt=re.compile(r'20.*'),
                    finishedAt=re.compile(r'20.*'),
                    tasks=[
                        re.compile(r'%s-[0-9]+-.+' % job_id),
                    ]
                )
            if validate_response_content(item, item_exp):
                break
        else:
            raise Exception('Did not find expected job history response')
