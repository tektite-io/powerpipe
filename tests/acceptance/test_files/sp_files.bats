load "$LIB_BATS_ASSERT/load.bash"
load "$LIB_BATS_SUPPORT/load.bash"

### spvars file tests ###

@test "test variable resolution in workspace mod set from steampipe.spvars file" {
  cd $FILE_PATH/test_data/mods/test_workspace_mod_var_precedence_set_from_steampipe_spvars

  run powerpipe query run query.version --output csv
  # check the output - query should use the value of variable set from the steampipe.spvars
  # file ("v7.0.0") which will give the output:
# +--------+----------+--------+
# | reason | resource | status |
# +--------+----------+--------+
# | v7.0.0 | v7.0.0   | ok     |
# +--------+----------+--------+
  assert_output 'reason,resource,status
v7.0.0,v7.0.0,ok'
}

@test "test variable resolution in workspace mod set from explicit spvars file" {
  cd $FILE_PATH/test_data/mods/test_workspace_mod_var_set_from_explicit_spvars

  run powerpipe query run query.version --output csv --var-file='deps.spvars'
  # check the output - query should use the value of variable set from the explicit spvars
  # file ("v8.0.0") which will give the output:
# +--------+----------+--------+
# | reason | resource | status |
# +--------+----------+--------+
# | v8.0.0 | v8.0.0   | ok     |
# +--------+----------+--------+
  assert_output 'reason,resource,status
v8.0.0,v8.0.0,ok'
}

# @test "test variable resolution in dependency mod set from *.auto.spvars file" {
#   cd $FILE_PATH/test_data/mods/test_dependency_mod_var_set_from_auto.ppvars

#   run steampipe query dependency_vars_1.query.version --output csv
#   # check the output - query should use the value of variable set from the *.auto.ppvars 
#   # file ("v8.0.0") which will give the output:
# # +--------+----------+--------+
# # | reason | resource | status |
# # +--------+----------+--------+
# # | v8.0.0 | v8.0.0   | ok     |
# # +--------+----------+--------+
#   assert_output 'reason,resource,status
# v8.0.0,v8.0.0,ok'
# }

### precedence tests ###

@test "test variable resolution precedence in workspace mod set from command line(--var) and steampipe.spvars file and *.auto.spvars file" {
  cd $FILE_PATH/test_data/mods/test_workspace_mod_var_precedence_set_from_both_spvars

  run powerpipe query run query.version --output csv --var version="v5.0.0"
  # check the output - query should use the value of variable set from the command line --var flag("v5.0.0") over 
  # steampipe.spvars("v7.0.0") and *.auto.spvars file("v8.0.0") which will give the output:
# +--------+----------+--------+
# | reason | resource | status |
# +--------+----------+--------+
# | v5.0.0 | v5.0.0   | ok     |
# +--------+----------+--------+
  assert_output 'reason,resource,status
v5.0.0,v5.0.0,ok'
}

### mod.sp file tests ###

@test "test that mod.sp is not renamed after uninstalling mod" {
  cd $FILE_PATH/test_data/mods/local_mod_with_mod.sp_file

  run powerpipe mod install
  assert_success

  run powerpipe mod uninstall
  # check mod.sp file still exists and is not renamed
  run ls mod.sp
  assert_success
}

### test basic check and query working for mod.pp files ###

@test "query with default params and no params passed through CLI" {
  cd $FILE_PATH/test_data/mods/functionality_test_mod_sp
  run powerpipe query run query.query_params_with_all_defaults --output json

  # store the reason field in `content`
  content=$(echo $output | jq '.rows[0].reason')

  assert_equal "$content" '"default_parameter_1 default_parameter_2 default_parameter_3"'
}

@test "control with default params and no args passed in control" {
  cd $FILE_PATH/test_data/mods/functionality_test_mod_sp
  run powerpipe control run control.query_params_with_defaults_and_no_args --export test.json
  echo $output
  ls

  # store the reason field in `content` 
  content=$(cat test.json | jq '.controls[0].results[0].reason')

  assert_equal "$content" '"default_parameter_1 default_parameter_2 default_parameter_3"'
  rm -f test.json
}

@test "control with no default params and no args passed in control" {
  cd $FILE_PATH/test_data/mods/functionality_test_mod_sp
  run powerpipe control run control.query_params_with_no_defaults_and_no_args --output json

  # should return an error `failed to resolve value for 3 parameters`
  echo $output
  [ $(echo $output | grep "failed to resolve value for 3 parameters" | wc -l | tr -d ' ') -eq 0 ]
}
