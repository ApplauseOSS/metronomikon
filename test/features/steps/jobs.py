import common
import re
import time

from behave import *


@when('we request all jobs')
def step_impl(context):
    resp = common.send_api_request(context, 'GET', 'v1/jobs')
    context.response = resp


@then('we should see the expected jobs returned')
def step_impl(context):
    common.check_response_status(context.response, 200)
    expected_response = common.build_expected_job_response(context.table)
    common.assert_expected_response(context.response.json(), expected_response)


@when('we request a specific job')
def step_impl(context):
    job_id = context.table[0]['job']
    resp = common.send_api_request(context, 'GET', 'v1/jobs/%s' % job_id)
    context.response = resp


@then('we should see the expected job returned')
def step_impl(context):
    common.check_response_status(context.response, 200)
    expected_response = common.build_expected_job_response(context.table)[0]
    common.assert_expected_response(context.response.json(), expected_response)


@when('we delete a specific job')
def step_impl(context):
    context.job_id = context.table[0]['job']
    resp = common.send_api_request(context, 'DELETE', 'v1/jobs/%s' % context.job_id)
    context.response = resp


@then('we expect the job to be deleted')
@common.retry_step(sleep=1)
def step_impl(context):
    resp = common.send_api_request(context, 'GET', 'v1/jobs/%s' % context.job_id)
    common.check_response_status(resp, 404)


@then('we expect the job to not be found')
def step_impl(context):
    common.check_response_status(context.response, 404)


@when('we run a specific job')
def step_impl(context):
    context.job_id = context.table[0]['job']
    resp = common.send_api_request(context, 'POST', 'v1/jobs/%s/runs' % context.job_id)
    common.check_response_status(resp, 201)
    context.job_run_id = resp.json().get('id', None)


@then('we check that the job has completed')
@common.retry_step(max_attempts=30, sleep=2)
def step_impl(context):
    resp = common.send_api_request(context, 'GET', 'v1/jobs/%s/runs/%s' % (context.job_id, context.job_run_id))
    common.check_response_status(resp, 200)
    exp = dict(
        completedAt=re.compile(r'20.*'),
        createdAt=re.compile(r'20.*'),
        id=re.compile(r'%s-[0-9]+' % context.job_id),
        jobId=context.job_id,
        status='COMPLETED',
        tasks=[],
    )
    if not common.validate_response_content(resp.json(), exp):
        raise Exception('Did not find expected COMPLETED response')


@then('we check that the job has failed')
@common.retry_step(max_attempts=30, sleep=2)
def step_impl(context):
    resp = common.send_api_request(context, 'GET', 'v1/jobs/%s/runs/%s' % (context.job_id, context.job_run_id))
    common.check_response_status(resp, 200)
    exp = dict(
        completedAt=None,
        createdAt=re.compile(r'20.*'),
        id=re.compile(r'%s-[0-9]+' % context.job_id),
        jobId=context.job_id,
        status='FAILED',
        tasks=[],
    )
    if not common.validate_response_content(resp.json(), exp):
        raise Exception('Did not find expected FAILED response')


@then('we check that the completed run shows up in the list of job runs')
def step_impl(context):
    resp = common.send_api_request(context, 'GET', 'v1/jobs/%s/runs' % context.job_id)
    common.check_response_status(resp, 200)
    # Look for one matching item in the response
    for item in resp.json():
        exp = dict(
            completedAt=re.compile(r'20.*'),
            createdAt=re.compile(r'20.*'),
            id=re.compile(r'%s-[0-9]+' % context.job_id),
            jobId=context.job_id,
            status='COMPLETED',
            tasks=[
                re.compile(r'%s-[0-9]+-.+' % context.job_id),
            ],
        )
        if common.validate_response_content(item, exp):
            return True
    raise Exception('Job run ID not found in list of job runs')


@when('we fetch history for a specific job')
def step_impl(context):
    context.job_id = context.table[0]['job']
    context.response = common.send_api_request(context, 'GET', 'v1/jobs/%s?embed=history' % context.job_id)
    common.check_response_status(context.response, 200)


@then('we check that the job history has the expected output')
def step_impl(context):
    common.check_response_status(context.response, 200)
    # Put together our own 'table' for the single job
    namespace, name = context.job_id.split('.')
    tmp_table = [
        dict(
            namespace=namespace,
            name=name,
            failed=False,
        )
    ]
    common.shared_check_job_history_response([context.response.json()], tmp_table)


@when('we fetch history for all jobs')
def step_impl(context):
    # Purposeful delay to give all jobs a chance to run before we hit max retry attempts
    time.sleep(2)
    context.response = common.send_api_request(context, 'GET', 'v1/jobs?embed=history')
    common.check_response_status(context.response, 200)


@then('we check that the history for all jobs has the expected output')
def step_impl(context):
    common.check_response_status(context.response, 200)
    common.shared_check_job_history_response(context.response.json(), context.table)
