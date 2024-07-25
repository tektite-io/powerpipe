load "$LIB_BATS_ASSERT/load.bash"
load "$LIB_BATS_SUPPORT/load.bash"

############### QUERIES ###############

@test "query with default params and no params passed through CLI" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe query run query_params_with_all_defaults --output json

  # store the reason field in `content`
  content=$(echo $output | jq '.rows[0].reason')

  assert_equal "$content" '"default_parameter_1 default_parameter_2 default_parameter_3"'
}

@test "query with default params and some named params passed through CLI" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe query run query_params_with_all_defaults --arg p1=command_param_1 --output json

  # store the reason field in `content`
  content=$(echo $output | jq '.rows[0].reason')

  assert_equal "$content" '"command_param_1 default_parameter_2 default_parameter_3"'
}

@test "query with no default params and no params passed through CLI" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe query run query_params_with_no_defaults --output json

  assert_output --partial 'failed to resolve args'
}

@test "query with no default params and all params passed through CLI" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe query run query_params_with_all_defaults --arg p1=command_param_1 --arg p2=command_param_2 --arg p3=command_param_3 --output json

  # store the reason field in `content`
  content=$(echo $output | jq '.rows[0].reason')

  assert_equal "$content" '"command_param_1 command_param_2 command_param_3"'
}

@test "query specific array index from param - DISABLED" {
  # cd $FUNCTIONALITY_TEST_MOD
  # run steampipe query query.query_array_params_with_default --output json

  # # store the reason field in `content`
  # content=$(echo $output | jq '.rows[0].reason')

  # assert_equal "$content" '"default_p1_element_02"'
}

@test "query with invalid param syntax" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe query run query_map_params_with_default --output json --arg p1=foo

  # should return an error `invalid input syntax for type json`
  assert_output --partial 'invalid input syntax for type json'
  cd -
}

@test "query specific property from map param" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe query run query_map_params_with_default --output json

  # store the reason field in `content`
  content=$(echo $output | jq '.rows[0].reason')

  assert_equal "$content" '"default_property_value_01"'
}

############### CONTROLS ###############

@test "control with default params and no args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_no_args --export test.json
  echo $output
  ls

  # store the reason field in `content` 
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"default_parameter_1 default_parameter_2 default_parameter_3"'
  rm -f test.json
}

@test "control with default params and partial named args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_partial_named_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"default_parameter_1 command_parameter_2 default_parameter_3"'
  rm -f test.json
}

@test "control with default params and partial named args passed through CLI args" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_partial_named_args --export test.json --arg p2=foo

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"default_parameter_1 foo default_parameter_3"'
  rm -f test.json
}

@test "control with default params and partial positional args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_partial_positional_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 default_parameter_2 default_parameter_3"'
  rm -f test.json
}

@test "control with default params and all named args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_all_named_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 command_parameter_2 command_parameter_3"'
  rm -f test.json
}

@test "control with default params and all named args passed through multiple CLI args" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_all_named_args --export test.json --arg p2=foo --arg p3=bar --arg p1=abcd

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"abcd foo bar"'
  rm -f test.json
}

@test "control with default params and all positional args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_defaults_and_all_positional_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 command_parameter_2 command_parameter_3"'
  rm -f test.json
}

@test "control with no default params and no args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_no_defaults_and_no_args --output json

  # should return an error `failed to resolve value for 3 parameters`
  echo $output
  [ $(echo $output | grep "failed to resolve value for 3 parameters" | wc -l | tr -d ' ') -eq 0 ]
}

@test "control with no default params and all args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_with_no_defaults_with_named_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 command_parameter_2 command_parameter_3"'
  rm -f test.json
}

@test "control to access specific property from map" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_map_with_default --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"default_property_value_01"'
  rm -f test.json
}

@test "control with invaild args syntax passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_params_invalid_arg_syntax --output json

  # store the results field in `content`
  content=$(cat output.json | jq '.controls[0].results')

  # should return an error `invalid input syntax for type json`, so the results should be empty
  assert_equal "$content" ""
}

@test "control with inline sql with partial named args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_inline_sql_from_control_with_partial_named_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 default_parameter_2 command_parameter_3"'
  rm -f test.json
}

@test "control with inline sql with partial positional args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_inline_sql_from_control_with_partial_positional_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 command_parameter_2 default_parameter_3"'
  rm -f test.json
}

@test "control with inline sql with no args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_inline_sql_from_control_with_no_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"default_parameter_1 default_parameter_2 default_parameter_3"'
  rm -f test.json
}

@test "control with inline sql with all named args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_inline_sql_from_control_with_all_named_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 command_parameter_2 command_parameter_3"'
  rm -f test.json
}

@test "control with inline sql with all positional args passed in control" {
  cd $FUNCTIONALITY_TEST_MOD
  run powerpipe control run query_inline_sql_from_control_with_all_positional_args --export test.json

  # store the reason field in `content`
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"command_parameter_1 command_parameter_2 command_parameter_3"'
  rm -f test.json
}

